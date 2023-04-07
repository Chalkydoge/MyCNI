# Current bugs

1. Cannot load eBPF program on `veth ingress` by executing `tc filter replace dev [DEV] ingress handle 0x1 bpf da obj /bin/veth_ingress.bpf.o`, but could be done by executing `veth_ingress [DEV_IFINDEX]`.

2. Compile `IPAM module` into binary, and using it by delegating to CNI framework.

3. Interactactions with kuberlet.

## Highlights

1. Reducing transfering datapath by equipping netdev with eBPF program(redirect_peer or redirect_neigh).


## Demo Test

1. Add these conf into `/etc/cni/net.d/` like this below:
```js
{
  "cniVersion": "1.0.0",
  "name": "mycni",
  "type": "vxlan",
  "mode": "vxlan",
}
```

2. Run `make build` at the root directory of this project.

3. A binary file called `mycni` would be generated.

4. Copy the binary `mycni` to directory `/opt/cni/bin`

5. Then this CNI plugin should work.
