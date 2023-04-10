echo ${PWD}

cd ${PWD}/plugins/ipam/etcdmode/initpool 
ls

cd ../../../../bpfmap
ls

# remove old conf & replace
cd ../

if [ -f "/etc/cni/net.d/10-mynet.conf" ];then
    rm /etc/cni/net.d/10-mynet.conf
fi
cp 10-mynet.conf /etc/cni/net.d


# Remove old plugins and copy new one
if [ -f "/opt/cni/bin/etcdmode" ];then
    rm /opt/cni/bin/etcdmode
fi

if [ -f "/opt/cni/bin/vxlan" ];then
    rm /opt/cni/bin/vxlan
fi

cd bin

cp etcdmode /opt/cni/bin
cp vxlan /opt/cni/bin

# This will test whether IP allocator works?
# go test -v -run TestAllocateIP2Pod