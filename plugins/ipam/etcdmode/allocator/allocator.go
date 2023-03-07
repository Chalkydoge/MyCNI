package allocator

import (
	"fmt"

	"mycni/etcdwrap"
	"mycni/utils"

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
	return true, nil
}


// func AllocateIP2Pod(containerID, ifname string) (*current.IPConfig, error) {
// 	// 1. read the hostname, query etcd
// 	// find whether allocated subnet for this host
// 	// hostname := utils.GetHostName()
// 	hoststring, err := cli.GetKV(utils.GetHostPath())
// 	if err != nil {
// 		return nil, fmt.Errorf("Cannot get subnet for host %v", err)
// 	}

// 	// hostsubnet like: 10.1.1.0/28
// 	if hoststring == "" {
// 		hoststring, err = AllocateIP2Host(cli)
// 		if err != nil {
// 			return nil, fmt.Errorf("Error when allocate ip2host: %v", err)
// 		}
// 	}

// 	// Now we have host's subnet
// 	var hostsubnet *net.IPNet
// 	_, hostsubnet, err = net.ParseCIDR(hoststring)
// 	if err != nil {
// 		return nil, fmt.Errorf()
// 	}

// 	// result of reserved IP and gateway for container
// 	var reservedIP *net.IPNet
// 	var gw net.IP
// 	var gwip string

// 	id := containerID + '' + ifname
// 	allocatedIP := GetByID(id)
// 	if allocatedIP != "" {
// 		return nil, fmt.Errorf("%s has been allocated to container %s, device %s", allocatedIP, containerID, ifname)
// 	}

// 	// Now we allocate one IP for it.
// 	hostIPPool := cli.GetKV(utils.GetHostIPPoolPath())
// 	ips := strings.Split(hostIPPool, ';')
// 	if len(ips) <= 0 {
// 		return nil, fmt.Error("All ip address has been used under this host!")
// 	}

// 	// convert into ip.Net object
// 	newIp := net.ParseIP(ips[0]) // allocate for device
	
// 	// Then put it back
// 	cli.PutKV(utils.GetHostIPPoolPath() , ips[1: ])

// 	// get host gw
// 	gwpath := utils.GetHostGWPath()
// 	gwip, err = cli.GetKV(gwpath)
// 	if err != nil {
// 		return "", fmt.Errorf("Cannot get gateway! Error is: %v", err)
// 	}
// 	gw = net.ParseIP(gwip)

// 	reservedIP = &net.IPNet{IP: newIP, Mask: r.Subnet.Mask}
// 	// Address: net.IPNet
// 	// Gateway: net.IP
// 	// allocate ip for containerid + ifname
// 	ipconf := &current.IPConfig {
// 		Address: newIp,
// 		Gateway: gw,
// 	}

// 	return ipconf, nil
// }

// func ReleasePodIP() () {

// }