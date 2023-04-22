package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"mycni/bpfmap"
	"mycni/pkg/ip"
	"mycni/pkg/ipam"
	"mycni/tc"
	"mycni/utils"
	"os"
	"runtime"
	"time"

	"net"
	"os/exec"
	"strconv"
	"strings"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ns"
	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
	"github.com/vishvananda/netlink"
)

const MODE_VXLAN = 1

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

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
}

func main() {
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, bv.BuildString("vxlan"))
}

/*****************************Veth Part******************************/

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

// This func should handle everything needed inside the pod namespace
func setupContainerVeth(netns ns.NetNS, ifName string, mtu int, pr *current.Result) (*current.Interface, *current.Interface, error) {
	// The IPAM result will be something like IP=192.168.3.5/24, GW=192.168.3.1.
	// Next best thing would be to let it ARP but set interface to 192.168.3.5/32 and
	// add a route like "192.168.3.0/24 via 192.168.3.1 dev $ifName".
	// Unfortunately that won't work as the GW will be outside the interface's subnet.

	// Our solution is to configure the interface with 192.168.3.5/24, then delete the
	// "192.168.3.0/24 dev $ifName" route that was automatically added. Then we add
	// "192.168.3.1/32 dev $ifName" and "192.168.3.0/24 via 192.168.3.1 dev $ifName".
	// In other words we force all traffic to ARP via the gateway except for GW itself.

	// host veth end
	hostInterface := &current.Interface{}
	// pod veth end
	containerInterface := &current.Interface{}

	err := netns.Do(func(hostNS ns.NetNS) error {
		hostVeth, contVeth0, err := ip.SetupVeth(ifName, mtu, "", hostNS)
		if err != nil {
			return err
		}
		hostInterface.Name = hostVeth.Name
		hostInterface.Mac = hostVeth.HardwareAddr.String()
		containerInterface.Name = contVeth0.Name
		containerInterface.Mac = contVeth0.HardwareAddr.String()

		utils.Log("host veth mac is " + hostInterface.Mac)

		containerInterface.Sandbox = netns.Path()
		pr.Interfaces = []*current.Interface{hostInterface, containerInterface}

		contVeth, err := net.InterfaceByName(ifName)
		if err != nil {
			return fmt.Errorf("failed to look up %q: %v", ifName, err)
		}

		if err = ipam.ConfigureIface(ifName, pr); err != nil {
			return err
		}

		for _, ipc := range pr.IPs {
			// Delete the route that was automatically added
			route := netlink.Route{
				LinkIndex: contVeth.Index,
				Dst: &net.IPNet{
					IP:   ipc.Address.IP.Mask(ipc.Address.Mask),
					Mask: ipc.Address.Mask,
				},
				Scope: netlink.SCOPE_LINK,
			}

			if err := netlink.RouteDel(&route); err != nil {
				return fmt.Errorf("failed to delete route %v: %v", route, err)
			}

			// addrBits := 32
			// if ipc.Address.IP.To4() == nil {
			// 	addrBits = 128
			// }

			// for _, r := range []netlink.Route{
			// 	// Special route to Gateway
			// 	{
			// 		LinkIndex: contVeth.Index,
			// 		Dst: &net.IPNet{
			// 			IP:   ipc.Gateway,
			// 			Mask: net.CIDRMask(addrBits, addrBits),
			// 		},
			// 		Scope: netlink.SCOPE_LINK,
			// 		Src:   ipc.Address.IP,
			// 	},
			// 	// Routes to other pods(in the same subnet)
			// 	// 这里尝试换成cluster-cidr的地址(为了多个节点的通信)
			// 	{
			// 		LinkIndex: contVeth.Index,
			// 		Dst: &net.IPNet{
			// 			IP:   pr.Routes[0].Dst.IP,
			// 			Mask: pr.Routes[0].Dst.Mask,
			// 		},
			// 		Scope: netlink.SCOPE_UNIVERSE,
			// 		Gw:    ipc.Gateway,
			// 		Src:   ipc.Address.IP,
			// 	},
			// } {
			// 	if err := netlink.RouteAdd(&r); err != nil {
			// 		return fmt.Errorf("failed to add route %v: %v", r, err)
			// 	}
			// }
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	for i := 0; i < 10; i++ {
		time.Sleep(300 * time.Millisecond)
		repeat, err := netlink.LinkByName(hostInterface.Name)
		if err != nil {
			return nil, nil, err
		}
		currMac := repeat.Attrs().HardwareAddr.String()
		if currMac != hostInterface.Mac {
			hostInterface.Mac = currMac
			break
		}
	}
	return hostInterface, containerInterface, nil
}

func setupHostVeth(vethName string, pr *current.Result) error {
	// hostVeth moved namespaces and may have a new ifindex
	h, err := netlink.LinkByName(vethName)
	if err != nil {
		return fmt.Errorf("failed to lookup %q: %v", vethName, err)
	}

	for _, ipc := range pr.IPs {
		maskLen := 128
		if ipc.Address.IP.To4() != nil {
			maskLen = 32
		}

		ipn := &net.IPNet{
			IP:   ipc.Gateway,
			Mask: net.CIDRMask(maskLen, maskLen),
		}

		addr := &netlink.Addr{IPNet: ipn, Label: ""}
		if err = netlink.AddrAdd(h, addr); err != nil {
			return fmt.Errorf("failed to add IP addr (%#v) to veth: %v", ipn, err)
		}

		ipn = &net.IPNet{
			IP:   ipc.Address.IP,
			Mask: net.CIDRMask(maskLen, maskLen),
		}

		// dst happens to be the same as IP/net of host veth
		if err = ip.AddHostRoute(ipn, nil, h); err != nil && !os.IsExist(err) {
			return fmt.Errorf("failed to add route on host: %v", err)
		}
	}
	return nil
}

// Just for test only
func getContainerVeth(netns ns.NetNS, ifName string) (*netlink.Veth, error) {
	var podVeth *netlink.Veth
	err := netns.Do(func(hostNS ns.NetNS) error {
		tmp, err := netlink.LinkByName(ifName)
		if err != nil {
			return err
		}

		podVeth = tmp.(*netlink.Veth)
		return nil
	})
	return podVeth, err
}

// create ARP Entry
func createARPEntry(ip, mac, dev, ns_name string) error {
	cmd := fmt.Sprintf("ip netns exec %s arp -s %s %s -i %s", ns_name, ip, mac, dev)
	utils.Log(cmd)
	processInfo := exec.Command(
		"/bin/sh", "-c",
		cmd,
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
func SetARP(gatewayIP, deviceName, mac, nsName string) error {
	return createARPEntry(gatewayIP, mac, deviceName, nsName)
}

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

// Helpers
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

func UInt32ToInetIP(ip uint32) string {
	p0 := (ip >> 24) & 0xff
	p1 := (ip >> 16) & 0xff
	p2 := (ip >> 8) & 0xff
	p3 := (ip) & 0xff

	b0 := strconv.Itoa(int(p0))
	b1 := strconv.Itoa(int(p1))
	b2 := strconv.Itoa(int(p2))
	b3 := strconv.Itoa(int(p3))
	ips := []string{b0, b1, b2, b3}
	return strings.Join(ips, ".")
}

// set veth pair info into linux-container-map
func setVethPairInfo2LxcMap(podIP string, hostVeth, nsVeth *netlink.Veth) error {
	netip, _, err := net.ParseCIDR(podIP)
	if err != nil {
		return err
	}

	podIP = netip.String()
	key := InetIpToUInt32(podIP)

	hostVethIndex := uint32(hostVeth.Attrs().Index)
	hostVethMac := stuff8Byte(([]byte)(hostVeth.Attrs().HardwareAddr))
	nsVethIndex := uint32(nsVeth.Attrs().Index)
	nsVethMac := stuff8Byte(([]byte)(nsVeth.Attrs().HardwareAddr))

	_, err = bpfmap.CreateLxcMap()
	if err != nil {
		return err
	}

	return bpfmap.SetLxcMap(
		bpfmap.EndpointMapKey{IP: key},
		bpfmap.EndpointMapInfo{
			// pod net device index
			PodIfIndex: nsVethIndex,
			// host device index
			LXCIfIndex: hostVethIndex,
			PodVethMAC: nsVethMac,
			LXCVethMAC: hostVethMac,
		},
	)
}

// set podname - ip mapping
func setPodIP2PodMap(podname, podIP string) error {
	key := [8]byte{}
	pod_strlen := len(podname)
	for i := pod_strlen - 5; i < pod_strlen; i++ {
		key[(i - (pod_strlen - 5))] = byte(podname[i])
	}

	// Note: podIP has cidr mask!
	netip, _, err := net.ParseCIDR(podIP)
	if err != nil {
		return err
	}
	podIP = netip.String()

	value := InetIpToUInt32(podIP)
	_, err = bpfmap.CreatePodIPMap()
	if err != nil {
		return err
	}

	return bpfmap.SetPodIPMap(
		bpfmap.PodInfoKey{PodName: key},
		bpfmap.PodInfoValue{IP: value},
	)
}

func loadPodIP(podname string) (string, error) {
	key := [8]byte{}
	pod_strlen := len(podname)
	utils.Log("podname is " + podname)
	for i := pod_strlen - 5; i < pod_strlen; i++ {
		key[i-(pod_strlen-5)] = byte(podname[i])
	}

	res, err := bpfmap.GetKeyValueFromPodIPMap(bpfmap.PodInfoKey{PodName: key})
	if err != nil {
		return "", err
	}

	ip_str := UInt32ToInetIP(res.IP)
	return ip_str, nil
}

// set current vxlan id into map
func setVxlanInfo2NodeMap(vxlan *netlink.Vxlan) error {
	key := bpfmap.VirtualNetKey{
		NetType: MODE_VXLAN,
	}
	val := bpfmap.VirtualNetValue{
		IfIndex: uint32(vxlan.Attrs().Index),
	}

	return bpfmap.SetVxlanMap(key, val)
}

// attach bpf program to veth device
//
// note: veth ingress is binded with bpf prog
func attachBPF2Veth(veth *netlink.Veth) error {
	name := veth.Attrs().Name
	vethIngressPath := tc.GetVethIngressPath()
	return tc.AttachBPF2Device(name, vethIngressPath, tc.INGRESS)
}

func createVXLAN(dev string) (*netlink.Vxlan, error) {
	return ip.SetupVXLAN(dev, 1500)
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
		utils.Log("Start to exec ipam Mode #" + n.IPAM.Type)
		r, err := ipam.ExecAdd(n.IPAM.Type, args.StdinData)
		if err != nil {
			return err
		}
		// Assure that if ipam failed, everything is cleared
		defer func() {
			ipam.ExecDel(n.IPAM.Type, args.StdinData)
		}()
		ipamRes, err := current.NewResultFromResult(r)
		if err != nil {
			return err
		}

		// Configure the container hardware address and IP address(es)
		result.IPs = ipamRes.IPs
		result.Routes = ipamRes.Routes
	}
	utils.Log("IPAM plugin success.")
	utils.Log(fmt.Sprintf("ipam res is %v", result))

	if len(result.IPs) == 0 {
		return errors.New("IPAM plugin returned missing IP config")
	}

	// get res from IPAM plugin
	ipConf := result.IPs[0]
	allocatedIPCIDR := ipConf.Address.String()
	gwIP := ipConf.Gateway.String()

	// setup netns
	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", args.Netns, err)
	}
	defer netns.Close()

	hostInterface, containerInterface, err := setupContainerVeth(netns, args.IfName, 1450, result)
	if err != nil {
		return err
	}

	var tmpMac string
	for i := 0; i < 5; i++ {
		hostv, err := netlink.LinkByName(hostInterface.Name)
		if err != nil {
			return err
		}
		utils.Log(fmt.Sprintf("Repeat %d/5 times, got mac addr %s", i+1, hostv.Attrs().HardwareAddr.String()))
		tmpMac = hostv.Attrs().HardwareAddr.String()
	}

	// Re-fetch newly built veth device
	hostv, err := netlink.LinkByName(hostInterface.Name)
	if err != nil {
		return err
	}
	podv, err := getContainerVeth(netns, containerInterface.Name)
	if err != nil {
		return err
	}

	// Then write mac-ip mapping into bpf map
	setVethPairInfo2LxcMap(allocatedIPCIDR, hostv.(*netlink.Veth), podv)
	utils.Log("Setup veth-ingress bpf mapping complete!")

	// Last set arp
	// Get the last item(ns name) from given path
	tmp := strings.Split(args.Netns, "/")
	err = SetARP(gwIP, args.IfName, tmpMac, tmp[len(tmp)-1])
	if err != nil {
		return err
	}
	utils.Log("ARP set complete!")

	// Finally attach bpf to tc ingress
	err = attachBPF2Veth(hostv.(*netlink.Veth))
	if err != nil {
		return err
	}
	utils.Log("veth BPF attach complete!")

	// For multinodes, we need tunnel between different nodes
	vxlan, err := createVXLAN("vxlan2")
	if err != nil {
		return err
	}
	utils.Log("vxlan setup complete!")

	err = attachBPF2VXLAN(vxlan)
	if err != nil {
		return err
	}
	utils.Log("attach bpf to vxlan in/egress complete!")

	err = setVxlanInfo2NodeMap(vxlan)
	if err != nil {
		return err
	}
	utils.Log("vxlan info written to bpfmap")

	return types.PrintResult(result, cniVersion)
}

func cmdCheck(args *skel.CmdArgs) error {
	return nil
}

// 这里把bpfmap中 lxcmap的部分给放在删除设备的时候一起执行
// podname-IP的映射多余了
func cmdDel(args *skel.CmdArgs) error {
	n, _, err := loadNetConf(args.StdinData, args.Args)
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}

	// Then, ipam exec del
	if err := ipam.ExecDel(n.IPAM.Type, args.StdinData); err != nil {
		return err
	}

	if args.Netns == "" {
		return nil
	}

	// There is a netns so try to clean up. Delete can be called multiple times
	// so don't return an error if the device is already removed.
	var ipnets []*net.IPNet
	err = ns.WithNetNSPath(args.Netns, func(_ ns.NetNS) error {
		var err error
		ipnets, err = ip.DelLinkByNameAddr(args.IfName)
		if err != nil && err == ip.ErrLinkNotFound {
			return nil
		}
		// Then del bpf map entry
		// IP from plugin captured result
		// Remove entry by this IP
		for _, ipnet := range ipnets {
			podIP := ipnet.IP.String()
			utils.Log("Previously allocated IP is " + podIP)
			bpfmap.DelKeyLxcMap(bpfmap.EndpointMapKey{
				IP: InetIpToUInt32(podIP),
			})
		}
		return err
	})

	// If there exists any ip net => return err
	if len(ipnets) != 0 {
		return err
	}
	return nil
}

// command used by cilium:
// tc filter replace dev [DEV] [ingress/egress] handle 1 bpf da obj [OBJ] sec [SEC]
// but this could not work when reading eBPF maps?
