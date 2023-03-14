package vxlan

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"math/rand"
	"mycni/pkg/ipam"

	"github.com/vishvananda/netlink"
	"github.com/containernetworking/plugins/pkg/ns"
)

type NetConf struct {
	types.NetConf
	BrName       string `json:"bridge"`
	IsGW         bool   `json:"isGateway"`
	IsDefaultGW  bool   `json:"isDefaultGateway"`
	ForceAddress bool   `json:"forceAddress"`
	IPMasq       bool   `json:"ipMasq"`
	MTU          int    `json:"mtu"`
	HairpinMode  bool   `json:"hairpinMode"`
	PromiscMode  bool   `json:"promiscMode"`
	Vlan         int    `json:"vlan"`
	MacSpoofChk  bool   `json:"macspoofchk,omitempty"`
	EnableDad    bool   `json:"enabledad,omitempty"`

	Args struct {
		Cni BridgeArgs `json:"cni,omitempty"`
	} `json:"args,omitempty"`
	RuntimeConfig struct {
		Mac string `json:"mac,omitempty"`
	} `json:"runtimeConfig,omitempty"`

	mac string
}

// cni interface needs:
// 		Name: dev.Attrs().Name,
// 		Mac:  dev.Attrs().HardwareAddr.String(),

// RandomVethName returns string "veth" with random prefix (hashed from entropy)
func RandomVethName() (string, error) {
	entropy := make([]byte, 4)
	_, err := rand.Read(entropy)
	if err != nil {
		return "", fmt.Errorf("failed to generate random veth name: %v", err)
	}

	// NetworkManager (recent versions) will ignore veth devices that start with "veth"
	return fmt.Sprintf("veth%x", entropy), nil
}

// Add route records (netinfo, gatewayip, device, scope)
func AddRoute(ipn *net.IPNet, gw net.IP, dev netlink.Link, scope ...netlink.Scope) error {
	defaultScope := netlink.SCOPE_UNIVERSE
	if len(scope) > 0 {
		defaultScope = scope[0]
	}
	return netlink.RouteAdd(&netlink.Route{
		LinkIndex: dev.Attrs().Index,
		Scope:     defaultScope,
		Dst:       ipn,
		Gw:        gw,
	})
} 

// delete netlink devices by name
func delInterfaceByName(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}

	if err = netlink.LinkDel(link); err != nil {
		return fmt.Errorf("failed to delete interface: %v", err)
	}

	return nil
}

// load net conf
func loadNetConf(bytes []byte, envArgs string) (*NetConf, string, error) {
	n := &NetConf{
		BrName: defaultBrName,
	}
	if err := json.Unmarshal(bytes, n); err != nil {
		return nil, "", fmt.Errorf("failed to load netconf: %v", err)
	}
	if n.Vlan < 0 || n.Vlan > 4094 {
		return nil, "", fmt.Errorf("invalid VLAN ID %d (must be between 0 and 4094)", n.Vlan)
	}

	if envArgs != "" {
		e := MacEnvArgs{}
		if err := types.LoadArgs(envArgs, &e); err != nil {
			return nil, "", err
		}

		if e.MAC != "" {
			n.mac = string(e.MAC)
		}
	}

	if mac := n.Args.Cni.Mac; mac != "" {
		n.mac = mac
	}

	if mac := n.RuntimeConfig.Mac; mac != "" {
		n.mac = mac
	}

	return n, n.CNIVersion, nil
}

/*****************************************************************//

// remove network interface by name
func removeVethPair(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}

	if err = netlink.LinkDel(link); err != nil {
		return fmt.Errorf("failed to delete veth pair: %v", err)
	}

	return nil
}

// create a veth pair with ifname, mtu and hostname
func createVethPair(name, peername string, mtu int) (*netlink.Veth, *netlink.Veth, error) {	
	// Find peer name that is not used
	if peername == "" {
		for {
			_vname, err := RandomVethName()
			peername = _vname
			if err != nil {
				return nil, nil, err
			}
	
			_, err = netlink.LinkByName(peername)
			if err != nil && !os.IsExist(err) {
				break
			}
		}
	}
	
	// if cannot get peername
	if peername == "" {
		return nil, nil, fmt.Errorf("create veth pair's name error")
	}

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: name,
			MTU:  mtu,
			// Namespace: netlink.NsFd(int(ns.Fd())),
		},
		PeerName:	peername,
		// PeerNamespace: netlink.NsFd(int(hostNS.Fd())),
	}

	// make veth pairs
	if err := netlink.LinkAdd(veth); err != nil {
		return nil, nil, err
	}

	// Re-fetch the conbtainer link veth, see whether it is success
	veth1, err := netlink.LinkByName(name) // veth1 shoule be in pod's netns
	if err != nil {
		// if not ok, delete
		netlink.LinkDel(veth1)
		return nil, nil, err
	}

	// Re-fetch the host link veth, see whether it is success
	veth2, err := netlink.LinkByName(peername) // veth2 on host
	if err != nil {
		// if not ok, delete
		netlink.LinkDel(veth2)
		return nil, nil, err
	}

	return veth1.(*netlink.Veth), veth2.(*netlink.Veth), nil
}

// remove veth pair made for hostside and netside
func removeHostVethPair(name string) (error) {
	err := removeVethPair(name)
	if err != nil {
		return err
	}
	return nil
}

// create veth pair for hostside and netside
func createHostVethPair() (*netlink.Veth, *netlink.Veth, error)  {
	hostVeth, _ := netlink.LinkByName("veth_host")
	netVeth, _ := netlink.LinkByName("veth_net")

	if hostVeth != nil && netVeth != nil {
		return hostVeth.(*netlink.Veth), netVeth.(*netlink.Veth), nil
	}
	return createVethPair("veth_host", "veth_net", 1500)
}

// setup veth device
func setupVeth(veth ...*netlink.Veth) error {
	for _, v := range veth {
		// start up veth devices
		err := netlink.LinkSetUp(v)
		if err != nil {
			return err
		}
	}
	return nil
}

// setup every host node's veth pair
func setupHostVethPair(veth ...*netlink.Veth) error {
	for _, v := range veth {
		err := setupVeth(v)
		if err != nil {
			return err
		}
	}
	return nil
}

// set veth end to be in hostNS
func setHostVethIntoHost(veth *netlink.Veth, netns ns.NetNS) error {
	err := netlink.LinkSetNsFd(veth, int(netns.Fd()))
	if err != nil {
		return fmt.Errorf("failed to add the device %q to ns: %v", device.Attrs().Name, err)
	}
	return nil
}

/****************************VXLAN part****************************/

// get ns info with given ns name
func getNetNS(_ns string) (*ns.NetNS, error) {
	netns, err := ns.GetNS(_ns)
	if err != nil {
		return nil, err
	}
	return &netns, nil
}

// set ip addr for vxlan
func setIPForVxlan(name, ipcidr string) error {
	deviceType := "vxlan"
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("failed to get %s device by name %q, error: %v", deviceType, name, err)
	}

	ipaddr, ipnet, err := net.ParseCIDR(ipcidr)
	if err != nil {
		return fmt.Errorf("failed to transform the ip %q, error : %v", ip, err)
	}

	// ipnet.IP = ipaddr
	err = netlink.AddrAdd(link, &netlink.Addr{IPNet: ipnet})
	if err != nil {
		return fmt.Errorf("can not add the ip %q to %s device %q, error: %v", ip, deviceType, name, err)
	}
	return nil	
}

// veth exists ip?
func deviceExistsIP(link *netlink.Veth) (string, error) {
	dev, err := net.InterfaceByIndex(link.Attrs().Index)
	if err != nil {
		return "", err
	}

	addrs, err := dev.Addrs()
	if err != nil {
		return "", err
	}
	if len(addrs) > 0 {
		firstIPCIDR := addrs[0].String()
		tmp := strings.Split(firstIPCIDR, "/")
		if len(tmp) == 2 && net.ParseIP(tmp[0]).To4() != nil {
			return firstIPCIDR, nil
		}
	}
	return "", nil
}

// set ip address into host veth endpoint
func setIPIntoHostPair(gatewayIP string, veth *netlink.Veth) (string, error) {
	// if already exists
	if ipExist, err := deviceExistsIP(veth); err == nil && ipExist != "" {
		return ipExist, nil
	}
	if err != nil {
		return "", err
	}

	// special 32 bit mask host
	gatewayIPCIDR = fmt.Sprintf("%s/%s", gatewayIP, 32)
	
	// set this special ip address for veth's vxlan
	return gatewayIPCIDR, setIPForVxlan(veth.Name, gatewayIPCIDR)
}

// Note: given pod ip string is not in cidr form
// setup ip address into pod veth endpoint
func setIpIntoPodPair(podIP string, veth *netlink.Veth) (string, error) {
	// special treatment
	podIPCIDR = fmt.Sprintf("%s/%s", podIP, 32)
	err = setIPForVxlan(veth.Name, podIPCIDR)
	if err != nil {
		return "", err
	}
	return podIPCIDR
}

// create veth pair inside given ns, from args
func createNsVethPair(args *skel.CmdArgs) (error) {
	mtu := 1450 // ethernet 14 bytes, ip 20 bytes, udp 8 bytes, vxlan tags: 8 bytes => 50 bytes used, atmost 1450 bytes for payload
	ifname := args.IfName
	hostname = "test_pod_" + RandomVethName()

	// curr, other, mtu
	return createVethPair(ifname, hostname, mtu)
}

// set default forward information
func setFIBTableIntoNS(gatewayIPCIDR string, veth *netlink.Veth) error {
	gatewayIP, gatewayNet, err := net.ParseCIDR(gatewayIPCIDR)
	if err != nil {
		return err
	}

	// create exchange route record
	defaultIP, defaultNet, err := net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		return err
	}

	// net ip device scope
	err = AddRoute(gatewayNet, defaultIP, veth, netlink.SCOPE_LINK)
	if err != nil {
		return err
	}

	err = AddRoute(defaultNet, gatewayIP, veth)
	if err != nil {
		return err
	}
	return nil
}

// create ARP Entry
func CreateARPEntry(ip, mac, dev string) error {
	processInfo := exec.Command(
		"/bin/sh", "-c",
		fmt.Sprintf("arp -s %s %s -i %s", ip, mac, dev)
	)
	_, err := processInfo.Output()
	return err
}

// delete ARP Entry
func DeleteARPEntry(ip, dev string) error {
	processInfo := exec.Command(
		"/bin/sh", "-c",
		fmt.Sprintf("arp -d %s -i %s", ip, dev)
	)
	_, err := processInfo.Output()
	return err
}

// set arp record, gatewayIP & gateway MAC for every pod ns devices
func setARP(gatewayIP, deviceName string, hostNS ns.NetNS, veth *netlink.Veth) error {
	err := hostns.Do(func() error {
		v, err := netlink.LinkByName(veth.Attrs().Name)
		if err != nil {
			return err
		}

		veth = v.(*netlink.Veth)
		mac := veth.LinkAttrs.HardwareAddr
		_mac := mac.String()
		
		return newNS.Do(func(hostNS net.NetNS) error {
			return CreateARPEntry(gatewayIP, _mac, deviceName)
		})
	})

	return err
}

func createVXLAN(name string) (*netlink.Vxlan, error) {
	defaultmtu := 1500

	// If already exists vxlan link...
	l, _ := netlink.LinkByName(name)
	vxlan, ok := l.(*netlink.Vxlan)
	if ok && vxlan != nil {
		return vxlan, nil
	}

	processInfo := exec.Command(
		"/bin/sh", "-c",
		fmt.Sprintf("ip link add name %s type vxlan external", name)
	)

	_, err := processInfo.Output()
	if err != nil {
		return nil, err
	}

	l, err = netlink.LinkByName(name)
	if err != nil {
		return nil, err
	}

	vxlan, ok := l.(*netlink.Vxlan)
	if !ok {
		return nil, fmt.Errorf("found the device %q but it's not a vxlan", name)
	}

	if err = netlink.LinkSetUp(vxlan); err != nil {
		return nil, fmt.Errorf("Setup VXLAN %s failed! error is %v", name, err)
	}
	return vxlan, nil
}

/*****************************************************/

// command Add, setup vxlan with given ipam & args
func cmdAdd(args *skel.CmdArgs) error {
	// 1. init ipam plugin
	var success bool = false
	n, cniVersion, err := loadNetConf(args.StdinData, args.Args)
	if err != nil {
		return err
	}

	// Assume L2 interface only
	result := &current.Result{
		CNIVersion: current.ImplementedSpecVersion,
		Interfaces: []*current.Interface{

		},
	}

	// need ipam?
	isLayer3 := (n.IPAM.Type != "")
	if (isLayer3) {
		r, err := ipam.ExecAdd(n.IPAM.Type, args.StdinData)
		if err != nil {
			return err
		}
		// Assure that if ipam failed, everything is cleared
		defer func() {
			if !success {
				ipam.ExecDel(n.IPAM.Type, args.StdinData)
			}
		}()

		ipamRes, err := current.NewResultFromResult(r)
		if err != nil {
			return err
		}

		result.IPS = ipamRes.IPS
		result.Routes = {} // empty because we didn't realize it
		result.DNS = {}

		// Configure the container hardware address and IP address(es)
	}

	// 2. after ipam, create a veth pair, veth_host and veth_net as gateway pair
	gatewaypair, netpair, err := createHostVethPair()
	if err != nil {
		return nil, err
	}

	// setup netns
	netns, err := GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", args.Netns, err)
	}
	defer netns.Close()

	// setup these devices
	err = setupHostVethPair(gatewaypair, netpair)
	if err != nil {
		return nil, err
	}

	// cidr /32 means only one address in this network
	// special ip for gateway
	// result.IPS contains both address & gateway

	// gatewayIP is like: '10.1.1.1/32'
	gatewayIP, err := setIpIntoHostPair(result.IPS[0].Gateway, gatewaypair)
	if err != nil {
		return nil, err
	}

	/* Inside every pod, init the network */
	var podPair, hostPair *netlink.Veth
	var podIP string

	// enter pod ns, do the follow things
	err = (*netns).Do(func(hostNS ns.NetNS) error {
		// create a veth pair, one for pod and one for host
		podPair, hostPair, err = createNsVethPair(args, pluginConfig)
		if err != nil {
			return err
		}

		// add host veth pair into kubelet ns
		err = setHostVethIntoHost(hostPair, hostNS)
		if err != nil {
			return err
		}
		
		// set pod veth's end to be gateway(/32)
		// todo: etcd will acknowledge other nodes
		podIP, err = setIpIntoPodPair(result.IPS[0].Address.IP, podPair)
		if err != nil {
			return err
		}

		// boot up pod NS veth ep
		err = setupVeth(podPair)
		if err != nil {
			return err
		}
		
		// FIB: forward information base
		// mapping between [mac addr -> ports]
		// layer 2 traffic to available ports
		err = setFIBTableIntoNS(gatewayIP, podPair)
		if err != nil {
			return err
		}

		// then set arp info for layer 2 traffic
		// Note: gatewayIP is with /mask here! need to remove!
		pureGatewayIP := string.Splits(gatewayIP, "/")
		err = setARP(pureGatewayIP[0], args.IfName, hostNS, hostPair)
		if err != nil {
			return err
		}

		// then boot host veth end
		err = setupHostPair(hostNS, hostPair)
		if err != nil {
			return err
		}

		// write veth pair info LINUX_CONTAINER_MAP / LXC_MAP_DEFAULT_PATH
		// err = setVethPairInfoToLxcMap(bpfmap, hostNs, podIP, hostPair, nsPair)
		// if err != nil {
		//   return err
		// }

		// Write veth pair ip, node ip mapping into NODE_LOCAL_MAP_DEFAULT_PATH
		return nil
	})

	if err != nil {
		return nil, err
	}

	// attach bpf to host veth tc ingress
	// err = attachBPF2Veth(hostPair)
	// if err != nil {
	// 	return nil, err
	// }

	// create a vxlan device
	vxlan, err := createVXLAN("any_vxlan_name_you_like")
	if err != nil {
		return nil, err
	}

	// TODO!
	// Add vxlan info into BPFMAP/NODE_LOCAL_MAP_DEFAULT_PATH
	// err = setVxlanInfoToLocalMap(bpfmap, vxlan)
	// if err != nil {
	// 	return nil, err
	// }

	// attach BPF to both VXLAN device, tc's ingress and egress
	// err = attachTcBPFIntoVxlan(vxlan)
	// if err != nil {
	// 	return nil, err
	// }

	// Finally for CNI result output,
	// since last part we have these information, we don't need to do it again
	
	// _gatewayIP, _, _ := net.ParseCIDR(gatewayIP)
	// _, _podIPNet, _ := net.ParseCIDR(podIP)
	// result := &types.Result{
	// 	CNIVersion: pluginConfig.CNIVersion,
	// 	IPs: []*types.IPConfig{
	// 		{
	// 			Address: *_podIPNet,
	// 			Gateway: _gatewayIP,
	// 		},
	// 	},
	// }
	
	return result, nil	
}