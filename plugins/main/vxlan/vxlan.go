package vxlan

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"mycni/bpfmap"
	"mycni/pkg/ipam"
	"mycni/tc"

	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

type NetConf struct {
	types.NetConf

	// Add a runtime config
	// usage: Netconf has an item: capabilities
	// cap {'aaa': true, 'bbb': false}, so aaa is acted & b is not
	// by export CAP_ARGS = {'aaa': false, 'bbb': true}, user can close and open some abilities.

	// RuntimeConfig struct {
	// 	// like setting default mac address
	// 	Mac string `json:"mac,omitempty"`
	// } `json:"runtimeConfig,omitempty"`

	podname   string
	namespace string
}

type K8SEnvArgs struct {
	types.CommonArgs
	K8S_POD_NAMESPACE types.UnmarshallableString `json:"K8S_POD_NAMESPACE,omitempty"`
	K8S_POD_NAME      types.UnmarshallableString `json:"K8S_POD_NAME,omitempty"`
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
	// first create an empty config
	n := &NetConf{}

	if err := json.Unmarshal(bytes, n); err != nil {
		return nil, "", fmt.Errorf("failed to load netconf: %v", err)
	}

	if envArgs != "" {
		e := K8SEnvArgs{}
		if err := types.LoadArgs(envArgs, &e); err != nil {
			return nil, "", err
		}

		// Loading some k8s arguments
		if e.K8S_POD_NAME != "" {
			n.podname = string(e.K8S_POD_NAME)
		}
		if e.K8S_POD_NAMESPACE != "" {
			n.namespace = string(e.K8S_POD_NAMESPACE)
		}
	}

	// if mac := n.RuntimeConfig.Mac; mac != "" {
	// 	n.mac = mac
	// }

	return n, n.CNIVersion, nil
}

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
		PeerName: peername,
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
func removeHostVethPair(name string) error {
	err := removeVethPair(name)
	if err != nil {
		return err
	}
	return nil
}

// create veth pair for hostside and netside
func createHostVethPair() (*netlink.Veth, *netlink.Veth, error) {
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
		return fmt.Errorf("failed to add the device %q to ns: %v", veth.Attrs().Name, err)
	}
	return nil
}

/****************************VXLAN part****************************/

// set ip addr for vxlan
func setIPForVxlan(name, ipcidr string) error {
	deviceType := "vxlan"
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("failed to get %s device by name %q, error is: %v", deviceType, name, err)
	}

	ipaddr, ipnet, err := net.ParseCIDR(ipcidr)
	if err != nil {
		return fmt.Errorf("failed to transform the ip %q, error is: %v", ipaddr, err)
	}

	// ipnet.IP = ipaddr
	err = netlink.AddrAdd(link, &netlink.Addr{IPNet: ipnet})
	if err != nil {
		return fmt.Errorf("can not add the ip %q to %s device %q, error: %v", ipaddr, deviceType, name, err)
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
	ipExist, err := deviceExistsIP(veth)
	if err != nil {
		return "", err
	}
	if ipExist != "" {
		return ipExist, nil
	}

	// special 32 bit mask host
	gatewayIPCIDR := fmt.Sprintf("%s/%s", gatewayIP, "32")

	// set this special ip address for veth's vxlan
	return gatewayIPCIDR, setIPForVxlan(veth.Name, gatewayIPCIDR)
}

// Note: given pod ip string is not in cidr form
// setup ip address into pod veth endpoint
func setIPIntoPodPair(podIP string, veth *netlink.Veth) (string, error) {
	// special treatment
	podIPCIDR := fmt.Sprintf("%s/%s", podIP, "32")
	err := setIPForVxlan(veth.Name, podIPCIDR)
	if err != nil {
		return "", err
	}
	return podIPCIDR, nil
}

// create veth pair inside given ns, from args
func createNsVethPair(args *skel.CmdArgs) (*netlink.Veth, *netlink.Veth, error) {
	mtu := 1450 // ethernet 14 bytes, ip 20 bytes, udp 8 bytes, vxlan tags: 8 bytes => 50 bytes used, atmost 1450 bytes for payload
	ifname := args.IfName
	randomName, err := RandomVethName()
	if err != nil {
		return nil, nil, fmt.Errorf("Cannot generate random veth name because of %v", err)
	}
	hostname := "test_pod_" + randomName

	// curr-endpoint, other-endpoint, mtu
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
		fmt.Sprintf("arp -s %s %s -i %s", ip, mac, dev),
	)
	_, err := processInfo.Output()
	return err
}

// delete ARP Entry
func DeleteARPEntry(ip, dev string) error {
	processInfo := exec.Command(
		"/bin/sh", "-c",
		fmt.Sprintf("arp -d %s -i %s", ip, dev),
	)
	_, err := processInfo.Output()
	return err
}

// set arp record, gatewayIP & gateway MAC for every pod ns devices
func setARP(gatewayIP, deviceName string, hostNS ns.NetNS, veth *netlink.Veth) error {
	err := hostNS.Do(func(newNS ns.NetNS) error {
		v, err := netlink.LinkByName(veth.Attrs().Name)
		if err != nil {
			return err
		}

		veth = v.(*netlink.Veth)
		mac := veth.LinkAttrs.HardwareAddr
		_mac := mac.String()

		return newNS.Do(func(hostNS ns.NetNS) error {
			return CreateARPEntry(gatewayIP, _mac, deviceName)
		})
	})

	return err
}

func createVXLAN(name string) (*netlink.Vxlan, error) {
	// defaultmtu := 1500
	// If already exists vxlan link...
	l, _ := netlink.LinkByName(name)
	vxlan, ok := l.(*netlink.Vxlan)
	if ok && vxlan != nil {
		return vxlan, nil
	}

	processInfo := exec.Command(
		"/bin/sh", "-c",
		fmt.Sprintf("ip link add name %s type vxlan external", name),
	)

	_, err := processInfo.Output()
	if err != nil {
		return nil, err
	}

	l, err = netlink.LinkByName(name)
	if err != nil {
		return nil, err
	}

	vxlan, ok = l.(*netlink.Vxlan)
	if !ok {
		return nil, fmt.Errorf("found the device %q but it's not a vxlan", name)
	}

	if err = netlink.LinkSetUp(vxlan); err != nil {
		return nil, fmt.Errorf("Setup VXLAN %s failed! error is %v", name, err)
	}
	return vxlan, nil
}

/*******************BPF MAP***************************/

func stuff8Byte(b []byte) [8]byte {
	var res [8]byte
	if len(b) > 8 {
		b = b[0:9]
	}

	for index, _byte := range b {
		res[index] = _byte
	}
	return res
}

func InetIpToUInt32(ip string) uint32 {
	bits := strings.Split(ip, ".")
	b0, _ := strconv.Atoi(bits[0])
	b1, _ := strconv.Atoi(bits[1])
	b2, _ := strconv.Atoi(bits[2])
	b3, _ := strconv.Atoi(bits[3])
	var sum uint32
	sum += uint32(b0) << 24
	sum += uint32(b1) << 16
	sum += uint32(b2) << 8
	sum += uint32(b3)
	return sum
}

// set veth pair info into linux-container-map
func setVethPairInfo2LxcMap(hostNS ns.NetNS, podIP string, hostVeth, nsVeth *netlink.Veth) error {
	err := hostNS.Do(func(newNS ns.NetNS) error {
		v, err := netlink.LinkByName(hostVeth.Attrs().Name)
		if err != nil {
			return err
		}
		hostVeth = v.(*netlink.Veth)
		return nil
	})
	if err != nil {
		return err
	}

	netip, _, err := net.ParseCIDR(podIP)
	if err != nil {
		return err
	}

	podIP = netip.String()

	hostVethIndex := uint32(hostVeth.Attrs().Index)
	hostVethMac := stuff8Byte(([]byte)(hostVeth.Attrs().HardwareAddr))
	nsVethIndex := uint32(nsVeth.Attrs().Index)
	nsVethMac := stuff8Byte(([]byte)(nsVeth.Attrs().HardwareAddr))

	_, err = bpfmap.CreateLxcMap()
	if err != nil {
		return err
	}

	return bpfmap.SetLxcMap(
		bpfmap.EndpointMapKey{},
		bpfmap.EndpointMapInfo{
			// pod net device index
			IfIndex: nsVethIndex,
			// host device index
			LXCIfIndex: hostVethIndex,
			MAC:        nsVethMac,
			NodeMAC:    hostVethMac,
		},
	)
}

// attach bpf program to veth device
//
// note: veth ingress is binded with bpf prog
func attachBPF2Veth(veth *netlink.Veth) error {
	name := veth.Attrs().Name
	vethIngressPath := tc.GetVethIngressPath()
	return tc.AttachBPF2Device(name, vethIngressPath, tc.INGRESS)
}

// attach bpf prog to vxlan(both ingress and egress)
func attachBPF2VXLAN(vxlan *netlink.Vxlan) error {
	name := vxlan.Attrs().Name
	ingressPath := tc.GetVxlanIngressPath()
	err := tc.AttachBPF2Device(name, ingressPath, tc.INGRESS)
	if err != nil {
		return err
	}

	egressPath := tc.GetVxlanEgressPath()
	return tc.AttachBPF2Device(name, egressPath, tc.EGRESS)
}

/*****************************************************/

// command Add, setup vxlan with given ipam & args
func cmdAdd(args *skel.CmdArgs) error {
	// 1. init ipam plugin
	var success bool = false

	// args.Args like:
	// "Args: "K8S_POD_INFRA_CONTAINER_ID=308102901b7fe9538fcfc71669d505bc09f9def5eb05adeddb73a948bb4b2c8b;
	// 		   K8S_POD_UID=d392609d-6aa2-4757-9745-b85d35e3d326;
	//		   IgnoreUnknown=1;
	//         K8S_POD_NAMESPACE=kube-system;
	//         K8S_POD_NAME=coredns-c676cc86f-4kz2t","
	n, cniVersion, err := loadNetConf(args.StdinData, args.Args)
	if err != nil {
		return err
	}

	// Assume L2 interface only
	result := &current.Result{
		CNIVersion: current.ImplementedSpecVersion,
		Interfaces: []*current.Interface{}, // nothing here
	}

	// need ipam?
	isLayer3 := (n.IPAM.Type != "")
	if isLayer3 {
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

		// Configure the container hardware address and IP address(es)
		result.IPs = ipamRes.IPs
	}

	// 2. after ipam, create a veth pair, veth_host and veth_net as gateway pair
	gatewaypair, netpair, err := createHostVethPair()
	if err != nil {
		return err
	}

	// setup netns
	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", args.Netns, err)
	}
	defer netns.Close()

	// setup these devices
	err = setupHostVethPair(gatewaypair, netpair)
	if err != nil {
		return err
	}

	// cidr /32 means only one address in this network
	// special ip for gateway
	// result.IPS contains both address & gateway

	// gatewayIP is like: '10.1.1.1/32'
	// IPConfig

	gatewayIPString := (result.IPs[0].Gateway).String() // is a net.IP object
	gatewayIP, err := setIPIntoHostPair(gatewayIPString, gatewaypair)
	if err != nil {
		return err
	}

	/* Inside every pod, init the network */
	var podPair, hostPair *netlink.Veth
	var podIPString string

	// enter pod ns, do the follow things
	err = netns.Do(func(hostNS ns.NetNS) error {
		// create a veth pair, one for pod and one for host
		podPair, hostPair, err = createNsVethPair(args)
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

		podIPString = result.IPs[0].Address.String() // is an ipnet
		if err != nil {
			return err
		}
		_, err = setIPIntoPodPair(podIPString, podPair)
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
		// pureGatewayIP := strings.Split(gatewayIP, "/")
		err = setARP(gatewayIPString, args.IfName, hostNS, hostPair)
		if err != nil {
			return err
		}

		// then boot host veth end
		err = hostNS.Do(func(newNS ns.NetNS) error {
			v, err := netlink.LinkByName(hostPair.Attrs().Name)
			if err != nil {
				return err
			}
			tmpVeth := v.(*netlink.Veth)
			return setupVeth(tmpVeth)
		})

		if err != nil {
			return err
		}

		// write veth pair info LINUX_CONTAINER_MAP / LXC_MAP_DEFAULT_PATH
		err = setVethPairInfo2LxcMap(hostNS, podIPString, hostPair, podPair)
		if err != nil {
			return err
		}

		// Write veth pair ip, node ip mapping into NODE_LOCAL_MAP_DEFAULT_PATH
		return nil
	})

	// attach bpf to host veth tc ingress
	err = attachBPF2Veth(hostPair)
	if err != nil {
		return err
	}

	// create a vxlan device
	vxlan, err := createVXLAN("my_vxlan")
	if err != nil {
		return err
	}

	err = attachBPF2VXLAN(vxlan)
	if err != nil {
		return err
	}
	// TODO!
	// Add vxlan info into BPFMAP/NODE_LOCAL_MAP_DEFAULT_PATH
	// err = setVxlanInfoToLocalMap(bpfmap, vxlan)
	// if err != nil {
	// 	return nil, err
	// }

	// Finally for CNI result output,
	// since last part we have these information, we don't need to do it again
	_gatewayIP, _, _ := net.ParseCIDR(gatewayIP)
	_, _podIPNet, _ := net.ParseCIDR(podIPString)

	podInterfaceIndex := (podPair.Attrs().Index)
	result.IPs = append(result.IPs, &current.IPConfig{
		Interface: &podInterfaceIndex,
		Address:   *_podIPNet,
		Gateway:   _gatewayIP,
	})
	success = true
	return types.PrintResult(result, cniVersion)
}

func cmdCheck() error {
	return nil
}
