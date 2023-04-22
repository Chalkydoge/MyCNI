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

struct nodeInfo {
    __u32 node_cidr; // cidr belongs to which node
};

struct nodeValue {
    __u32 node_ip; // target node's real ip
}

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 64);
    __type(key, struct nodeInfo);
    __type(value, struct nodeValue);
    __uint(pinning, LIBBPF_PIN_BY_NAME);
} node_map SEC(".maps");

SEC("classifier")
int vxlan_egress(struct __sk_buff *ctx)
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
    
    // Lookup target node info with given ip
    __u32 wrapped_ip = (dst_ip & (~0xFF));

    struct nodeInfo *targetNode = bpf_map_lookup_elem(&node_map, &wrapped_ip);
    // given ip belongs to some pod in the cluster
    if (targetNode) {
        // exist inside node_map => pods on same node
        // rewrite mac addr to src:[lxc mac] and dst:[pod mac]
        // then, redirect the packet to target pod's lxc
        __u32 dst_node_ip = targetNode->ip;

        // preparing a bpf tunnel
        struct bpf_tunnel_key key;
        int ret;
        __builtin_memset(&key, 0x0, sizeof(key));

        key.remote_ipv4 = targetNode->ip;
        key.tunnel_id = DEFAULT_TUNNEL_ID;
        key.tunnel_tos = 0;
        key.tunnel_ttl = 64;
        
        ret = bpf_skb_set_tunnel_key(skb, &key, sizeof(key), BPF_F_ZERO_CSUM_TX);
        if (ret < 0) {
            bpf_printk("bpf_skb_set_tunnel_key has failed");
            return TC_ACT_SHOT;
        }
        return TC_ACT_OK;
    }
    
    // If the ip is not inside cluster, do nothing!
    return TC_ACT_OK;
}

char __license[] SEC("license") = "GPL";
