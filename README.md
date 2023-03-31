# Current bugs

1. Cannot load eBPF program on `veth ingress` by executing `tc filter replace dev [DEV] ingress handle 0x1 bpf da obj /bin/veth_ingress.bpf.o`, but could be done by executing `veth_ingress [DEV_IFINDEX]`.

2. Compile `IPAM module` into binary, and using it by delegating to CNI framework.

3. Interactactions with kuberlet.

## Highlights

1. Reducing transfering datapath by equipping netdev with eBPF program(redirect_peer or redirect_neigh).


