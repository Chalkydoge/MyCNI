package initpool

import (
	"fmt"
	"strings"

	"mycni/utils"
	"mycni/etcdwrap"
)

// Generate a list of ip, like 10.1.1.0/28,  10.1.2.0/28, ...
func RandomGenerateIpCIDRs() []string {
	ans := make([]string, 16)
	for i := 0; i < 16; i++ {
		ans[i] = fmt.Sprintf("10.1.%d.0/28", (i + 1))
	}
	return ans
}

// Init ip cidr pool(for all host in this cluster)
func InitPool(cli *etcdwrap.WrappedClient) (bool, error) {
	ips := RandomGenerateIpCIDRs()
	ipCIDRs := strings.Join(ips, ";")
	
	if cli.GetInitPoolStatus() == false {
		err := cli.PutKV(utils.GetIPPoolPath(), ipCIDRs)
		if (err != nil) {
			return false, fmt.Errorf("Cannot add ipcidr into pool! %v", err)
		}
		cli.SetInitPoolStatus(true)
	}
	return true, nil
}

// Release ip cidr pool
func ReleasePool(cli *etcdwrap.WrappedClient) (bool, error) {
	err := cli.DelKV(utils.GetIPPoolPath())
	if err != nil {
		return false, fmt.Errorf("Release ip pool failed! Error is %v", err)
	}
	cli.SetInitPoolStatus(false)
	return true, nil
}