// SPDX-License-Identifier: (LGPL-2.1 OR BSD-2-Clause)
/* Copyright (c) 2023 */
#include <vmlinux.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>

//  First, let's copy a template from `tc.bpf.c` as a good start,
#define TC_ACT_UNSPEC   (-1)
#define TC_ACT_OK	0
#define ETH_P_IP	0x0800		/* Internet Protocol packet	*/
#define ETH_ALEN    6           /* Ethernet Address len*/

struct epInfo {
    __u32 lxc_ifindex;       // inside host
    __u32 pod_ifindex;       // inside pod
    __u8  lxc_mac[8];        // veth pair lxc, 2 bytes for padding
    __u8  pod_mac[8];        // veth pair mac, 2 bytes for padding
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 10);
    __type(key, __u32);
    __type(value, struct epInfo);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} lxc_map SEC(".maps");

SEC("classifier") // bind to the section of 'tc'
int veth_ingress(struct __sk_buff *ctx)
{
	void *data_end = (void *)(__u64)ctx->data_end;
	void *data = (void *)(__u64)ctx->data;
	struct ethhdr *l2;
	struct iphdr *l3;
    
    bpf_printk("loaded");

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
    unsigned char src_mac[ETH_ALEN] = {0xee, 0xb5, 0x4d, 0x5a, 0xed, 0xaf};
    unsigned char dst_mac[ETH_ALEN] = {0x42, 0x7a, 0x42, 0x79, 0xf2, 0xbe};
    
    // Lookup ep info with dest ip    
    struct epInfo *ep = bpf_map_lookup_elem(&lxc_map, &dst_ip);    
    bpf_printk("Looking up inside bpf map with key=%d", dst_ip);
    bpf_printk("BPF Map obj null? %d", (&lxc_map != NULL));
    bpf_printk("ep info: %d", (ep != NULL));

    if (ep) {
        // exist inside lxc_map => pods on same node
        // rewrite mac addr to src:[lxc mac] and dst:[pod mac]
        // then, redirect the packet to target pod's lxc
        __u32 redirect_ifindex = ep->lxc_ifindex;

        bpf_printk("Redirect packet to device #%d", redirect_ifindex);

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

    // Then, the packet is not to pod on same node, handling to host's gateway
    // Not implemented yet...	
    return TC_ACT_OK;
}

char __license[] SEC("license") = "GPL";
