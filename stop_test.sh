if [ -f "/etc/cni/net.d/10-mynet.conf" ];then
    rm /etc/cni/net.d/10-mynet.conf
    echo "CNI conf removed!"
fi

echo "Stop test complete!"