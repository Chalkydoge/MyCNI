# EtcdMode IPAM

A simple ipam using `k8s-etcd` as local storage inside cluster.

> allocator: allocates ip from k8s-Node, as well a static IP-Pool for newly added Pods ON THIS NODE(so automatically these pods are in same subnet of this Node)

> etcdwrap: a wrapped etcdclient(singleton) for performing R/W etcd

> main.go: the entry point of CNI-IPAM part. read the conf and check whether type hits 'etcdmode', if hits, read the IPAM part config, do the following things:

When Calling `cmdAdd`:
- Init host pool if not ready
- Allocate one IP from pool to the given device
- Mark the relationship between given device & IP

When Calling `cmdDel`:
- Remove pod-ip relations under this host if any
- Recycle theses ips back to the host IP pool
- If the host is done, return back the host's IP Subnet back to IP Subnet Pool for the cluster

When Calling `cmdCheck`:
- Check CNI version matches?
- Looking around if there is at least one IP address allocated to the container
