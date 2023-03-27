package vxlan

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"mycni/bpfmap"
	"mycni/pkg/ip"
	"mycni/pkg/ipam"
	"mycni/tc"
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

func init() {
	// this ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()
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
		log.Print("host veth mac is ", hostInterface.Mac)

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
				Scope: netlink.SCOPE_NOWHERE,
			}

			if err := netlink.RouteDel(&route); err != nil {
				return fmt.Errorf("failed to delete route %v: %v", route, err)
			}

			addrBits := 32
			if ipc.Address.IP.To4() == nil {
				addrBits = 128
			}

			for _, r := range []netlink.Route{
				// Special route to Gateway
				{
					LinkIndex: contVeth.Index,
					Dst: &net.IPNet{
						IP:   ipc.Gateway,
						Mask: net.CIDRMask(addrBits, addrBits),
					},
					Scope: netlink.SCOPE_LINK,
					Src:   ipc.Address.IP,
				},
				// Routes to other pods(in the same subnet)
				{
					LinkIndex: contVeth.Index,
					Dst: &net.IPNet{
						IP:   ipc.Address.IP.Mask(ipc.Address.Mask),
						Mask: ipc.Address.Mask,
					},
					Scope: netlink.SCOPE_UNIVERSE,
					Gw:    ipc.Gateway,
					Src:   ipc.Address.IP,
				},
			} {
				if err := netlink.RouteAdd(&r); err != nil {
					return fmt.Errorf("failed to add route %v: %v", r, err)
				}
			}
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

/****************************VXLAN part*****************************/

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
		log.Print("Mac addr is ", _mac)
		return newNS.Do(func(ns.NetNS) error {
			return CreateARPEntry(gatewayIP, _mac, deviceName)
		})
	})

	return err
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
func setVethPairInfo2LxcMap(podIP string, hostVeth, nsVeth *netlink.Veth) error {
	netip, _, err := net.ParseCIDR(podIP)
	if err != nil {
		return err
	}

	podIP = netip.String()
	key := InetIpToUInt32(podIP)
	log.Println(hostVeth.Attrs().Index)
	log.Println(nsVeth.Attrs().Index)

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
		log.Print("Start to exec ipam...(ip address is automatically recycled!)")
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
		log.Print("IPAM result is", ipamRes)
	}

	log.Print("IPAM end")

	if len(result.IPs) == 0 {
		return errors.New("IPAM plugin returned missing IP config")
	}

	// setup netns
	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", args.Netns, err)
	}
	defer netns.Close()

	hostInterface, _, err := setupContainerVeth(netns, args.IfName, 1450, result)
	if err != nil {
		return err
	}

	if err = setupHostVeth(hostInterface.Name, result); err != nil {
		return err
	}

	return types.PrintResult(result, cniVersion)
}

func cmdCheck() error {
	return nil
}

func cmdDel() error {
	return nil
}
