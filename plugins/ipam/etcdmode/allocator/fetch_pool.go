package getters

import (
	"fmt"
)

func GetOneIPFromPool(poolKey string, cli *etcdwrap.WrappedClient) (string, error) {
	val, err := cli.GetKV(poolKey)
	if err != nil {
		// handle error
		return nil, err
	}

	ippool := utils.ConvertString2Array(val)
	ans := ippool[0]

	// update new pool
	err = cli.PutKV(poolKey, utils.ConvertArray2String(ippool[1: ]))
	if err != nil {
		return ans, err
	}
	return ans, nil
}

func AllocateIP2Host(cli *etcdwrap.WrappedClient) (string, error) {
	// 0. Find out whether host has been allocated an IP
	hostip, err := cli.GetKV(utils.GetHostPath())
	if hostip != nil {
		return hostip, nil
	}

	// 1. fetch an ip cidr from ip pool
	ip, err := GetOneIPFromPool(utils.GetIPPoolPath(), cli)
	if err != nil {
		return "", fmt.Errorf("Cannot get ip from ip pool, msg: %v", err)
	}

	// 2. write into etcd 
	// mycni/ipam/<hostname> = ip
	err = cli.PutKV(GetHostPath(), ip)
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
	
	// 4. last, setup host's local ip pool
	hostPoolPath := utils.GetHostIPPoolPath()
	cli.PutKV(hostPoolPath, utils.GetValidIps(ip))
}

func AllocateIP2Pod(containerID, ifname string) (*current.IPConfig, error) {
	// returns a types100 result

	// 1. read the hostname, query etcd
	// find whether allocated subnet for this host
	// hostname := utils.GetHostName()
	hostsubnet, err := cli.GetKV(utils.GetHostPath())
	if err != nil {
		return nil, fmt.Errorf("Cannot get subnet for host %v", err)
	}

	// hostsubnet like: 10.1.1.0/28
	if hostsubnet == "" {
		hostsubnet, err = AllocateIP2Host(cli)
		if err != nil {
			return nil, fmt.Errorf("Error when allocate ip2host: %v", err)
		}
	}

	// result of reserved IP and gateway for container
	var reservedIP *net.IPNet
	var gw net.IP

	id := containerID + '' + ifname
	allocatedIP := GetByID(id)
	if allocatedIP != "" {
		return nil, fmt.Errorf("%s has been allocated to container %s, device %s", allocatedIP, containerID, ifname)
	}

	// Now we allocate one IP for it.
	hostIPPool := cli.GetKV("mycni/ipam/<hostname>/pool")

	ips := strings.Split(hostIPPool, ';')
	if len(ips) <= 0 {
		return nil, fmt.Error("All ip address has been used under this host!")
	}

	newIp := ips[0] // allocate for device
	// Then put it back
	cli.PutKV("mycni/ipam/<hostname>/pool" , ips[1: ])

	// convert into ip.Net object

	// allocate ip for containerid + ifname
	ipconf := &current.IPConfig {
		Address: newIp,
		Gateway: gw,
	}

	return ipconf, nil
}

