
PWD=$(pwd)
BIN_PATH=$PWD"/../../../bin"
VETH_INGRESS_BPF_PATH=$BIN_PATH"/veth_ingress.bpf.o"

# get veth device
ip l | grep veth

# then attach bpf program to veth end
tc qdisc add dev vethf4ce9639 clsact
tc filter replace dev vethf4ce9639 ingress handle 0x1 bpf da obj veth_ingress.bpf.o

tc qdisc add dev veth0c00619e clsact
tc filter replace dev veth0c00619e ingress handle 0x1 bpf da obj veth_ingress.bpf.o

