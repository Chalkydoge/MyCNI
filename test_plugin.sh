echo ${PWD}

cd ${PWD}/plugins/ipam/etcdmode/initpool 
ls

cd ../../../../bpfmap
ls

go test -v -run TestIterate
go test -v -run TestResetMap

# remove old conf & replace
cd ../

if [ -f "/etc/cni/net.d/10-mynet.conf" ];then
    rm /etc/cni/net.d/10-mynet.conf
fi
cp 10-mynet.conf /etc/cni/net.d


# Remove old plugins and copy new one
if [ -f "/opt/cni/bin/local" ];then
    rm /opt/cni/bin/local
fi

if [ -f "/opt/cni/bin/vxlan" ];then
    rm /opt/cni/bin/vxlan
fi

if [ -f "/opt/cni/bin/veth_ingress.bpf.o" ];then
    rm /opt/cni/bin/veth_ingress.bpf.o
fi

cd bin

cp local /opt/cni/bin
cp vxlan /opt/cni/bin
cp veth_ingress.bpf.o /opt/cni/bin

# This will test whether IP allocator works?
# go test -v -run TestAllocateIP2Pod