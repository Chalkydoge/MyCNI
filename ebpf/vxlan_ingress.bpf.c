// SPDX-License-Identifier: (LGPL-2.1 OR BSD-2-Clause)
/* Copyright (c) 2023 */
#include <vmlinux.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

//  First, let's copy a template from `tc.bpf.c` as a good start,
#define TC_ACT_UNSPEC   (-1)
#define TC_ACT_OK	0
#define TC_ACT_SHOT 1
#define ETH_P_IP	0x0800		/* Internet Protocol packet	*/
#define ETH_ALEN    6           /* Ethernet Address len*/
#define DEFAULT_TUNNEL_ID 13190

struct epInfo {
    __u32 lxc_ifindex;       // inside host
    __u32 pod_ifindex;       // inside pod
    __u8  lxc_mac[8];        // veth pair lxc, 2 bytes for padding
    __u8  pod_mac[8];        // veth pair mac, 2 bytes for padding
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 32);
    __type(key, __u32);
    __type(value, struct epInfo);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} lxc_map SEC(".maps");


SEC("classifier")
int vxlan_ingress(struct __sk_buff *ctx)
{
    // normal sanity checks
	void *data_end = (void *)(__u64)ctx->data_end;
	void *data = (void *)(__u64)ctx->data;
	struct ethhdr *l2;
	struct iphdr *l3;

    // non-ip protocol
	if (ctx->protocol != bpf_htons(ETH_P_IP))
		return TC_ACT_UNSPEC;

    // empty l2 frames
	l2 = data;
	if ((void *)(l2 + 1) > data_end)
		return TC_ACT_UNSPEC;

    // empty l3 packets
	l3 = (struct iphdr *)(l2 + 1);
	if ((void *)(l3 + 1) > data_end)
		return TC_ACT_UNSPEC;

    // Ensure that it's an ip packet(version unknown)
    __u32 src_ip = bpf_htonl(l3->saddr);
    __u32 dst_ip = bpf_htonl(l3->daddr);

    // layer2 mac address
    unsigned char src_mac[ETH_ALEN] = {0xab, 0xcd, 0xef, 0xab, 0xcd, 0xef};
    unsigned char dst_mac[ETH_ALEN] = {0xab, 0xcd, 0xef, 0xab, 0xcd, 0xef};
    
    // Almost same with `veth_ingress` except that it's attached to vxlan device
    struct epInfo *ep = bpf_map_lookup_elem(&lxc_map, &ep);
    if (ep) {
        // exist inside lxc_map => pods on same node
        // rewrite mac addr to src:[lxc mac] and dst:[pod mac]
        // then, redirect the packet to target pod's lxc
        __u32 redirect_ifindex = ep->lxc_ifindex;

        // load src mac, dst mac from ethhdr
        for (int i = 0; i < ETH_ALEN; i++) {
            src_mac[i] = l2->h_dest[i];
            dst_mac[i] = ep->lxc_mac[i]; // rewrite to veth endpoint veth
        }

        // bpf_printk("dst mac %x:%x:%x:%x:%x:%x", dst_mac[0], dst_mac[1], dst_mac[2], dst_mac[3], dst_mac[4], dst_mac[5]);

        bpf_skb_store_bytes(ctx, offsetof(struct ethhdr, h_source), src_mac, ETH_ALEN, 0);
        bpf_skb_store_bytes(ctx, offsetof(struct ethhdr, h_dest), dst_mac, ETH_ALEN, 0);
        
        return bpf_redirect_peer(ep->lxc_ifindex, 0);
    }
    
    // If vxlan received an unknown ip => drop
    return TC_ACT_OK;
}

char __license[] SEC("license") = "GPL";
