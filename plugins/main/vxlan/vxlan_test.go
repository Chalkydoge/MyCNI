package vxlan

import (
	"fmt"
	"mycni/bpfmap"
	"net"
	"testing"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/stretchr/testify/assert"
	"github.com/vishvananda/netlink"
)

func TestCmdAddPod1(t *testing.T) {
	te := assert.New(t)
	conf := fmt.Sprintf(`{
		"cniVersion": "%s",
		"name": "mynet",
		"type": "vxlan",
		"ipam": {
			"type": "etcdmode"
		}
	}`, "1.0.0")

	args := &skel.CmdArgs{
		ContainerID: "308102901b7fe9538fcfc71669d505bc09f9def5eb05adeddb73a948bb4b2c8b",
		Netns:       "/var/run/netns/ns1",
		IfName:      "eth0",
		Args:        "K8S_POD_NAMESPACE=kube-system;K8S_POD_NAME=coredns-c676cc86f-4kz2t",
		Path:        "/opt/cni/bin",
		StdinData:   []byte(conf),
	}

	// Load CNI config first
	n, cniVersion, err := loadNetConf(args.StdinData, args.Args)
	te.Nil(err)
	te.Equal(cniVersion, "1.0.0")
	te.Equal(n.IPAM.Type, "etcdmode")

	// Assume L2 interface only
	result := &current.Result{
		CNIVersion: current.ImplementedSpecVersion,
		Interfaces: []*current.Interface{}, // nothing here
	}

	// Assume result we have from ipam.ExecAdd is
	// etcdmode_test.go:45:
	// ipamRes := &{1.0.0 [] [{Interface:<nil> Address:{IP:10.1.1.6 Mask:fffffff0} Gateway:10.1.1.1}] [] {[]  [] []}}
	ipnet := net.IPNet{
		IP:   net.ParseIP("10.1.3.2"),
		Mask: net.IPv4Mask(0xff, 0xff, 0xff, 0xf0),
	}
	// Assume this is the ipam result, we have current ip net 10.1.1.6/28
	// and gateway ip = 10.1.1.1
	ipamRes := current.Result{
		CNIVersion: "1.0.0",
		Interfaces: []*current.Interface{},
		IPs: []*current.IPConfig{
			{
				Interface: nil,
				Address:   ipnet,
				Gateway:   net.ParseIP("10.1.3.1"),
			},
		},
		Routes: []*types.Route{},
		DNS: types.DNS{
			Nameservers: []string{},
			Domain:      "",
			Search:      []string{},
			Options:     []string{},
		},
	}

	// Configure the container hardware address and IP address(es)
	result.IPs = ipamRes.IPs

	if len(result.IPs) == 0 {
		t.Error("IPAM plugin returned missing IP config")
	}
	// setup netns
	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		t.Errorf("failed to open netns %q: %v", args.Netns, err)
	}
	defer netns.Close()

	hostInterface, containerInterface, err := setupContainerVeth(netns, args.IfName, 1450, result)
	if err != nil {
		t.Log(err)
	}

	if err = setupHostVeth(hostInterface.Name, result); err != nil {
		t.Log(err)
	}

	t.Log(result)

	for i := 0; i < 5; i++ {
		hostv, err := netlink.LinkByName(hostInterface.Name)
		if err != nil {
			t.Log(err)
		}
		t.Logf("Repeat %d/5 times, got mac addr %s", i+1, hostv.Attrs().HardwareAddr.String())
	}

	hostv, err := netlink.LinkByName(hostInterface.Name)
	if err != nil {
		t.Log(err)
	}
	podv, err := getContainerVeth(netns, containerInterface.Name)
	if err != nil {
		t.Log(err)
	}

	setVethPairInfo2LxcMap("10.1.3.2/28", hostv.(*netlink.Veth), podv)
	t.Log("BPF Map for pod1 complete!")

	mp, err := bpfmap.CreateLxcMap()
	if err != nil {
		t.Log(err)
	}

	epInfo := &bpfmap.EndpointMapInfo{}
	err = mp.Lookup(bpfmap.EndpointMapKey{IP: InetIpToUInt32("10.1.3.2")}, epInfo)
	if err != nil {
		t.Log(err)
	}
	t.Log(epInfo)
}

func TestCmdAddPod2(t *testing.T) {
	te := assert.New(t)
	conf := fmt.Sprintf(`{
		"cniVersion": "%s",
		"name": "mynet",
		"type": "vxlan",
		"ipam": {
			"type": "etcdmode"
		}
	}`, "1.0.0")

	args := &skel.CmdArgs{
		ContainerID: "30810114514fe9538fcfc71669d505bc09f9def5eb05adeddb73a948bb4b2c8b",
		Netns:       "/var/run/netns/ns2",
		IfName:      "eth0",
		Args:        "K8S_POD_NAMESPACE=kube-system;K8S_POD_NAME=coredns-c67ddc86f-4kz2t",
		Path:        "/opt/cni/bin",
		StdinData:   []byte(conf),
	}

	// Load CNI config first
	n, cniVersion, err := loadNetConf(args.StdinData, args.Args)
	te.Nil(err)
	te.Equal(cniVersion, "1.0.0")
	te.Equal(n.IPAM.Type, "etcdmode")

	// Assume L2 interface only
	result := &current.Result{
		CNIVersion: current.ImplementedSpecVersion,
		Interfaces: []*current.Interface{}, // nothing here
	}

	// Assume result we have from ipam.ExecAdd is
	// etcdmode_test.go:45:
	// ipamRes := &{1.0.0 [] [{Interface:<nil> Address:{IP:10.1.1.6 Mask:fffffff0} Gateway:10.1.1.1}] [] {[]  [] []}}
	ipnet := net.IPNet{
		IP:   net.ParseIP("10.1.3.3"),
		Mask: net.IPv4Mask(0xff, 0xff, 0xff, 0xf0),
	}
	// Assume this is the ipam result, we have current ip net 10.1.1.6/28
	// and gateway ip = 10.1.1.1
	ipamRes := current.Result{
		CNIVersion: "1.0.0",
		Interfaces: []*current.Interface{},
		IPs: []*current.IPConfig{
			{
				Interface: nil,
				Address:   ipnet,
				Gateway:   net.ParseIP("10.1.3.1"),
			},
		},
		Routes: []*types.Route{},
		DNS: types.DNS{
			Nameservers: []string{},
			Domain:      "",
			Search:      []string{},
			Options:     []string{},
		},
	}

	// Configure the container hardware address and IP address(es)
	result.IPs = ipamRes.IPs

	if len(result.IPs) == 0 {
		t.Error("IPAM plugin returned missing IP config")
	}
	// setup netns
	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		t.Errorf("failed to open netns %q: %v", args.Netns, err)
	}
	defer netns.Close()

	hostInterface, containerInterface, err := setupContainerVeth(netns, args.IfName, 1450, result)
	if err != nil {
		t.Log(err)
	}
	if err = setupHostVeth(hostInterface.Name, result); err != nil {
		t.Log(err)
	}

	for i := 0; i < 5; i++ {
		hostv, err := netlink.LinkByName(hostInterface.Name)
		if err != nil {
			t.Log(err)
		}
		t.Logf("Repeat %d/5 times, got mac addr %s", i+1, hostv.Attrs().HardwareAddr.String())
	}

	t.Log(result)

	hostv, err := netlink.LinkByName(hostInterface.Name)
	if err != nil {
		t.Log(err)
	}
	podv, err := getContainerVeth(netns, containerInterface.Name)
	if err != nil {
		t.Log(err)
	}

	setVethPairInfo2LxcMap("10.1.3.3/28", hostv.(*netlink.Veth), podv)
	t.Log("BPF Map for pod2 complete!")

	mp, err := bpfmap.CreateLxcMap()
	if err != nil {
		t.Log(err)
	}

	epInfo := &bpfmap.EndpointMapInfo{}
	err = mp.Lookup(bpfmap.EndpointMapKey{IP: InetIpToUInt32("10.1.3.3")}, epInfo)
	if err != nil {
		t.Log(err)
	}
	t.Log(epInfo)
}
