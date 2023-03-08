package allocator

import (
	"fmt"
	"net"
	"strings"

	"mycni/etcdwrap"
	"mycni/utils"
	current "github.com/containernetworking/cni/pkg/types/100"
)

func GetOneIPFromPool(poolKey string, cli *etcdwrap.WrappedClient) (string, error) {
	// Get Ip pool array from etcd first
	val, err := cli.GetKV(poolKey)
	if err != nil {
		// handle error
		return "", err
	}

	// Next fetch the first addr
	ippool := utils.ConvertString2Array(val)
	ans := ippool[0]

	// update new pool
	err = cli.PutKV(poolKey, utils.ConvertArray2String(ippool[1: ]))
	if err != nil {
		return ans, err
	}
	return ans, nil
}

// Allocate one IP subnet for host(node)
func AllocateIP2Host(cli *etcdwrap.WrappedClient) (string, error) {
	// 0. Find out whether host has been allocated an IP
	hostip, err := cli.GetKV(utils.GetHostPath())
	if err != nil {
		return "", fmt.Errorf("Error when getting host ip! err is %v", err)
	}
	if hostip != "" {
		return hostip, nil
	}

	// 1. fetch an ip cidr from ip pool
	ip, err := GetOneIPFromPool(utils.GetIPPoolPath(), cli)
	if err != nil {
		return "", fmt.Errorf("Cannot get ip from ip pool, msg: %v", err)
	}

	// 2. write into etcd 
	// mycni/ipam/<hostname> = ip, means that host has been allocated
	err = cli.PutKV(utils.GetHostPath(), ip)
	if err != nil {
		return "", fmt.Errorf("Cannot assign up to host! Error is: %v", err)
	}

	// 3. set host's gw
	gwip := utils.GetGateway(ip)
	gwpath := utils.GetHostGWPath()
	err = cli.PutKV(gwpath, gwip)
	if err != nil {
		return "", fmt.Errorf("Cannot assign gateway! Error is: %v", err)
	}
	
	// 4. last, setup host's local ip pool, put the string in
	var ips []string
	hostPoolPath := utils.GetHostIPPoolPath()
	ips, err = utils.GetValidIps(ip)
	if err != nil {
		return "", fmt.Errorf("Get valid ips failed! err is %v", err)
	}
	
	cli.PutKV(hostPoolPath,	utils.ConvertArray2String(ips))
	return ip, nil
}

// Release allocated Ip to the host, add it back to ip pool
func ReleaseHostIP(cli *etcdwrap.WrappedClient) (bool, error) {
	// 0. Find out whether host has been allocated an IP
	hostip, err := cli.GetKV(utils.GetHostPath())
	if err != nil {
		return false, fmt.Errorf("Error when getting host ip! err is %v", err)
	}
	if hostip == "" {
		// hostip has been empty/released
		return true, nil
	}

	// 1. Assume pods on this host has been ALL released,
	// if host has allocated ip subnet release it
	err = cli.DelKV(utils.GetHostPath())
	if err != nil {
		return false, fmt.Errorf("Error when del host ip! err is %v", err)
	}
	err = cli.DelKV(utils.GetHostGWPath())
	if err != nil {
		return false, fmt.Errorf("Error when del host's gateway ip! err is %v", err)
	}
	return true, nil
}

// Allocate ip under certain host, fetch one from ip pool then assign to special device
// Returns the IPConfig of CNI Standards
func AllocateIP2Pod(containerID, ifname string, cli *etcdwrap.WrappedClient) (*current.IPConfig, error) {
	// 1. read the hostname, query etcd
	// find whether allocated subnet for this host
	// hostname := utils.GetHostName()
	hoststring, err := cli.GetKV(utils.GetHostPath())
	if err != nil {
		return nil, fmt.Errorf("Cannot get subnet for host %v", err)
	}

	// hostsubnet like: 10.1.1.0/28
	if hoststring == "" {
		hoststring, err = AllocateIP2Host(cli)
		if err != nil {
			return nil, fmt.Errorf("Error when allocate ip2host: %v", err)
		}
	}

	// Now we have host's subnet
	var hostsubnet *net.IPNet
	_, hostsubnet, err = net.ParseCIDR(hoststring)
	if err != nil {
		return nil, fmt.Errorf("Cannot parse hoststring as valid ipcidr! err is %v", err)
	}

	// 2. get the result of reserved IP and gateway for container
	var reservedIP *net.IPNet
	var gw, newIp net.IP
	var gwip, allocatedIP string

	id := containerID + "-" + ifname
	allocatedIP, err = cli.GetKV(utils.GetNetDevicePath(id))
	if err != nil {
		return nil, fmt.Errorf("Cannot get current network device! err is %v", err)
	}
	if allocatedIP != "" {
		return nil, fmt.Errorf("%s has been allocated to container %s, device %s", allocatedIP, containerID, ifname)
	}

	// Now we allocate one IP for it
	var hostIPPool string
	hostIPPool, err = cli.GetKV(utils.GetHostIPPoolPath())
	ips := strings.Split(hostIPPool, ";")
	if len(ips) <= 0 {
		return nil, fmt.Errorf("All ip address has been used under this host!")
	}

	// convert into ip.Net object
	newIp, _, err = net.ParseCIDR(ips[0]) // allocate for device

	// Then put it back
	cli.PutKV(utils.GetHostIPPoolPath(), utils.ConvertArray2String(ips[1: ]))

	// get host gw
	gwpath := utils.GetHostGWPath()
	gwip, err = cli.GetKV(gwpath)
	if err != nil {
		return nil, fmt.Errorf("Cannot get gateway! Error is: %v", err)
	}
	gw, _, err = net.ParseCIDR(gwip)

	reservedIP = &net.IPNet{IP: newIp, Mask: hostsubnet.Mask}
	// Address: net.IPNet
	// Gateway: net.IP
	// allocate ip for containerid + ifname
	ipconf := &current.IPConfig {
		Address: *reservedIP,
		Gateway: gw,
	}

	// update current device's ip info into db
	err = cli.PutKV(utils.GetNetDevicePath(id), ips[0])
	if err != nil {
		return nil, fmt.Errorf("Error happened when writing new config into etcd! %v", err)
	}
	return ipconf, nil
}

// Release pod ip with given containerID, ifname in skel.Args
func ReleasePodIP(containerID, ifname string, cli *etcdwrap.WrappedClient) (bool, error) {
	// get the result of reserved IP and gateway for container
	id := containerID + "-" + ifname
	allocatedIP, err := cli.GetKV(utils.GetNetDevicePath(id))
	if err != nil {
		return false, fmt.Errorf("Cannot get current network device! err is %v", err)
	}

	// so if allocated ip is empty, do nothing
	if allocatedIP == "" {
		return true, nil
	}

	// Now we return back one IP for it.
	var hostIPPool string
	hostIPPool, err = cli.GetKV(utils.GetHostIPPoolPath())
	ips := strings.Split(hostIPPool, ";")
	// Then put it back	
	ips = append(ips, allocatedIP)
	
	// update back to hostpool
	cli.PutKV(utils.GetHostIPPoolPath(), utils.ConvertArray2String(ips))

	// update current device's ip info into db
	err = cli.DelKV(utils.GetNetDevicePath(id))
	if err != nil {
		return false, fmt.Errorf("Error happened when removing config for device %s! error is %v", id, err)
	}
	return true, nil
}

// Release Host Gateway item
func ReleaseHostGateway(cli *etcdwrap.WrappedClient) (bool, error) {
	err := cli.DelKV(utils.GetHostGWPath())
	if err != nil {
		return false, fmt.Errorf("Cannot release host gateway ip %v", err)
	}
	return true, nil
}