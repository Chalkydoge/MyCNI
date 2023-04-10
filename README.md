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


Invalid CIDR address(Nil): because IPAM is not working correctly! (IP Pool has been drained!)

```
Apr 09 10:13:52 master kubelet[642]: E0409 10:13:52.936119     642 kuberuntime_manager.go:782] "CreatePodSandbox for pod failed" err="rpc error: code = Unknown desc = failed to setup network for sandbox \"a71d65930ec04c2c6193bcb278cc4d1d37c5e89061aee5dbe89e4dc953d234ec\": plugin type=\"vxlan\" name=\"mycni\" failed (add): invalid CIDR address: <nil>" pod="kube-system/coredns-5bbd96d687-tvpxr"
Apr 09 10:13:52 master kubelet[642]: E0409 10:13:52.936222     642 pod_workers.go:965] "Error syncing pod, skipping" err="failed to \"CreatePodSandbox\" for \"coredns-5bbd96d687-tvpxr_kube-system(2e7c5af8-d056-4474-969b-b43283cc9f6c)\" with CreatePodSandboxError: \"Failed to create sandbox for pod \\\"coredns-5bbd96d687-tvpxr_kube-system(2e7c5af8-d056-4474-969b-b43283cc9f6c)\\\": rpc error: code = Unknown desc = failed to setup network for sandbox \\\"a71d65930ec04c2c6193bcb278cc4d1d37c5e89061aee5dbe89e4dc953d234ec\\\": plugin type=\\\"vxlan\\\" name=\\\"mycni\\\" failed (add): invalid CIDR address: <nil>\"" pod="kube-system/coredns-5bbd96d687-tvpxr" podUID=2e7c5af8-d056-4474-969b-b43283cc9f6c
Apr 09 10:13:54 master kubelet[642]: E0409 10:13:54.933997     642 remote_runtime.go:176] "RunPodSandbox from runtime service failed" err="rpc error: code = Unknown desc = failed to setup network for sandbox \"b9509afcdc5fb5c5ef559b86aa5286ef0adf5d3a4626e6ec27601c7db552e5f9\": plugin type=\"vxlan\" name=\"mycni\" failed (add): invalid CIDR address: <nil>"
```

Another bug after this: (because network is not connected)
```
Apr 09 10:34:10 master kubelet[642]: W0409 10:34:10.833931     642 logging.go:59] [core] [Channel #4 SubChannel #5] grpc: addrConn.createTransport failed to connect to {
Apr 09 10:34:10 master kubelet[642]:   "Addr": "/var/run/containerd/containerd.sock",
Apr 09 10:34:10 master kubelet[642]:   "ServerName": "/var/run/containerd/containerd.sock",
Apr 09 10:34:10 master kubelet[642]:   "Attributes": null,
Apr 09 10:34:10 master kubelet[642]:   "BalancerAttributes": null,
Apr 09 10:34:10 master kubelet[642]:   "Type": 0,
Apr 09 10:34:10 master kubelet[642]:   "Metadata": null
Apr 09 10:34:10 master kubelet[642]: }. Err: connection error: desc = "transport: Error while dialing dial unix /var/run/containerd/containerd.sock: connect: connection refused"
Apr 09 10:34:10 master kubelet[642]: E0409 10:34:10.834044     642 remote_image.go:212] "ImageFsInfo from image service failed" err="rpc error: code = Unavailable desc = connection error: desc = \"transport: Error while dialing dial unix /var/run/containerd/containerd.sock: connect: connection refused\""
Apr 09 10:34:10 master kubelet[642]: E0409 10:34:10.834060     642 eviction_manager.go:261] "Eviction manager: failed to get summary stats" err="failed to get imageFs stats: rpc error: code = Unavailable desc = connection error: desc = \"transport: Error while dialing dial unix /var/run/containerd/containerd.sock: connect: connection refused\""
```

But after fixing connectivity between pods, errors still exist.

Logs:
```
Apr 10 11:00:23 master kubelet[642]: I0410 11:00:23.032038     642 kubelet_volumes.go:160] "Cleaned up orphaned pod volumes dir" podUID=79a87307-5b6e-4fa0-b1d8-43345ec402bd path="/var/lib/kubelet/pods/79a87307-5b6e-4fa0-b1d8-43345ec402bd/volumes"
Apr 10 11:00:24 master kubelet[642]: E0410 11:00:24.084565     642 remote_runtime.go:176] "RunPodSandbox from runtime service failed" err="rpc error: code = Unavailable desc = error reading from server: EOF"
Apr 10 11:00:24 master kubelet[642]: E0410 11:00:24.084683     642 kuberuntime_sandbox.go:72] "Failed to create sandbox for pod" err="rpc error: code = Unavailable desc = error reading from server: EOF" pod="kube-system/coredns-5bbd96d687-nffpx"
Apr 10 11:00:24 master kubelet[642]: E0410 11:00:24.084715     642 kuberuntime_manager.go:782] "CreatePodSandbox for pod failed" err="rpc error: code = Unavailable desc = error reading from server: EOF" pod="kube-system/coredns-5bbd96d687-nffpx"
Apr 10 11:00:24 master kubelet[642]: E0410 11:00:24.084979     642 pod_workers.go:965] "Error syncing pod, skipping" err="failed to \"CreatePodSandbox\" for \"coredns-5bbd96d687-nffpx_kube-system(1a5e681e-355c-4737-ae7e-b0672667c60e)\" with CreatePodSandboxError: \"Failed to create sandbox for pod \\\"coredns-5bbd96d687-nffpx_kube-system(1a5e681e-355c-4737-ae7e-b0672667c60e)\\\": rpc error: code = Unavailable desc = error reading from server: EOF\"" pod="kube-system/coredns-5bbd96d687-nffpx" podUID=1a5e681e-355c-4737-ae7e-b0672667c60e
Apr 10 11:00:24 master kubelet[642]: E0410 11:00:24.085865     642 remote_runtime.go:176] "RunPodSandbox from runtime service failed" err="rpc error: code = Unavailable desc = error reading from server: EOF"
Apr 10 11:00:24 master kubelet[642]: E0410 11:00:24.085912     642 kuberuntime_sandbox.go:72] "Failed to create sandbox for pod" err="rpc error: code = Unavailable desc = error reading from server: EOF" pod="kube-system/coredns-5bbd96d687-bhmwz"
Apr 10 11:00:24 master kubelet[642]: E0410 11:00:24.085927     642 kuberuntime_manager.go:782] "CreatePodSandbox for pod failed" err="rpc error: code = Unavailable desc = error reading from server: EOF" pod="kube-system/coredns-5bbd96d687-bhmwz"
Apr 10 11:00:24 master kubelet[642]: E0410 11:00:24.085967     642 pod_workers.go:965] "Error syncing pod, skipping" err="failed to \"CreatePodSandbox\" for \"coredns-5bbd96d687-bhmwz_kube-system(87d04705-6687-4a0c-817f-d7ac59a65768)\" with CreatePodSandboxError: \"Failed to create sandbox for pod \\\"coredns-5bbd96d687-bhmwz_kube-system(87d04705-6687-4a0c-817f-d7ac59a65768)\\\": rpc error: code = Unavailable desc = error reading from server: EOF\"" pod="kube-system/coredns-5bbd96d687-bhmwz" podUID=87d04705-6687-4a0c-817f-d7ac59a65768
Apr 10 11:00:24 master kubelet[642]: W0410 11:00:24.552021     642 logging.go:59] [core] [Channel #1 SubChannel #2] grpc: addrConn.createTransport failed to connect to {
Apr 10 11:00:24 master kubelet[642]:   "Addr": "/var/run/containerd/containerd.sock",
Apr 10 11:00:24 master kubelet[642]:   "ServerName": "/var/run/containerd/containerd.sock",
Apr 10 11:00:24 master kubelet[642]:   "Attributes": null,
Apr 10 11:00:24 master kubelet[642]:   "BalancerAttributes": null,
Apr 10 11:00:24 master kubelet[642]:   "Type": 0,
Apr 10 11:00:24 master kubelet[642]:   "Metadata": null
Apr 10 11:00:24 master kubelet[642]: }. Err: connection error: desc = "transport: Error while dialing dial unix /var/run/containerd/containerd.sock: connect: connection refused"
Apr 10 11:00:24 master kubelet[642]: E0410 11:00:24.552145     642 remote_runtime.go:277] "ListPodSandbox with filter from runtime service failed" err="rpc error: code = Unavailable desc = connection error: desc = \"transport: Error while dialing dial unix /var/run/containerd/containerd.sock: connect: connection refused\"" filter="nil"
Apr 10 11:00:24 master kubelet[642]: E0410 11:00:24.552193     642 kuberuntime_sandbox.go:300] "Failed to list pod sandboxes" err="rpc error: code = Unavailable desc = connection error: desc = \"transport: Error while dialing dial unix /var/run/containerd/containerd.sock: connect: connection refused\""
Apr 10 11:00:24 master kubelet[642]: E0410 11:00:24.552725     642 generic.go:236] "GenericPLEG: Unable to retrieve pods" err="rpc error: code = Unavailable desc = connection error: desc = \"transport: Error while dialing dial unix /var/run/containerd/containerd.sock: connect: connection refused\""
Apr 10 11:00:25 master kubelet[642]: E0410 11:00:25.027308     642 remote_runtime.go:277] "ListPodSandbox with filter from runtime service failed" err="rpc error: code = Unavailable desc = connection error: desc = \"transport: Error while dialing dial unix /var/run/containerd/containerd.sock: connect: connection refused\"" filter="&PodSandboxFilter{Id:,State:&PodSandboxStateValue{State:SANDBOX_READY,},LabelSelector:map[string]string{},}"
```


Apr 10 16:14:50 master kubelet[12039]: E0410 16:14:50.904799   12039 kuberuntime_sandbox.go:300] "Failed to list pod sandboxes" err="rpc error: code = Unavailable desc = connection error: desc = \"transport: Error while dialing dial unix /var/run/containerd/containerd.sock: connect: connection refused\""
Apr 10 16:14:50 master kubelet[12039]: E0410 16:14:50.904832   12039 generic.go:236] "GenericPLEG: Unable to retrieve pods" err="rpc error: code = Unavailable desc = connection error: desc = \"transport: Error while dialing dial unix /var/run/containerd/containerd.sock: connect: connection refused\""
Apr 10 16:14:51 master kubelet[12039]: W0410 16:14:51.906145   12039 logging.go:59] [core] [Channel #1 SubChannel #2] grpc: addrConn.createTransport failed to connect to {
Apr 10 16:14:51 master kubelet[12039]:   "Addr": "/var/run/containerd/containerd.sock",
Apr 10 16:14:51 master kubelet[12039]:   "ServerName": "/var/run/containerd/containerd.sock",
Apr 10 16:14:51 master kubelet[12039]:   "Attributes": null,
Apr 10 16:14:51 master kubelet[12039]:   "BalancerAttributes": null,
Apr 10 16:14:51 master kubelet[12039]:   "Type": 0,
Apr 10 16:14:51 master kubelet[12039]:   "Metadata": null
Apr 10 16:14:51 master kubelet[12039]: }. Err: connection error: desc = "transport: Error while dialing dial unix /var/run/containerd/containerd.sock: connect: connection refused"
Apr 10 16:14:51 master kubelet[12039]: E0410 16:14:51.906859   12039 remote_runtime.go:277] "ListPodSandbox with filter from runtime service failed" err="rpc error: code = Unavailable desc = connection error: desc = \"transport: Error while dialing dial unix /var/run/containerd/containerd.sock: connect: connection refused\"" filter="&PodSandboxFilter{Id:,State:&PodSandboxStateValue{State:SANDBOX_READY,},LabelSelector:map[string]string{},}"
Apr 10 16:14:51 master kubelet[12039]: E0410 16:14:51.907017   12039 kuberuntime_sandbox.go:300] "Failed to list pod sandboxes" err="rpc error: code = Unavailable desc = connection error: desc = \"transport: Error while dialing dial unix /var/run/containerd/containerd.sock: connect: connection refused\""
Apr 10 16:14:51 master kubelet[12039]: E0410 16:14:51.907145   12039 kubelet_pods.go:1138] "Error listing containers" err="rpc error: code = Unavailable desc = connection error: desc = \"transport: Error while dialing dial unix /var/run/containerd/containerd.sock: connect: connection refused\""
Apr 10 16:14:51 master kubelet[12039]: E0410 16:14:51.907321   12039 kubelet.go:2283] "Failed cleaning pods" err="rpc error: code = Unavailable desc = connection error: desc = \"transport: Error while dialing dial unix /var/run/containerd/containerd.sock: connect: connection refused\""
Apr 10 16:14:51 master kubelet[12039]: E0410 16:14:51.907496   12039 remote_runtime.go:277] "ListPodSandbox with filter from runtime service failed" err="rpc error: code = Unavailable desc = connection error: desc = \"transport: Error while dialing dial unix /var/run/containerd/containerd.sock: connect: connection refused\"" filter="nil"
Apr 10 16:14:51 master kubelet[12039]: E0410 16:14:51.907622   12039 kuberuntime_sandbox.go:300] "Failed to list pod sandboxes" err="rpc error: code = Unavailable desc = connection error: desc = \"transport: Error while dialing dial unix /var/run/containerd/containerd.sock: connect: connection refused\""
Apr 10 16:14:51 master kubelet[12039]: E0410 16:14:51.907746   12039 generic.go:236] "GenericPLEG: Unable to retrieve pods" err="rpc error: code = Unavailable desc = connection error: desc = \"transport: Error while dialing dial unix /var/run/containerd/containerd.sock: connect: connection refused\""