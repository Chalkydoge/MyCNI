package vxlan

import (
	"fmt"
	"net"
	"testing"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/stretchr/testify/assert"
)

func TestRandomVethNames(t *testing.T) {
	te := assert.New(t)
	vethname, err := RandomVethName()
	te.Nil(err)
	t.Log("veth name is ", vethname)
}

func TestAddVethPairs(t *testing.T) {
	te := assert.New(t)
	netVeth, hostVeth, err := createHostVethPair()
	te.Nil(err)
	t.Log("Net veth conf: ", netVeth)
	t.Log("Host veth conf: ", hostVeth)
}

func TestRemoveHostVethPairs(t *testing.T) {
	te := assert.New(t)
	// only need to remove one end
	err := removeHostVethPair("veth_host")
	te.Nil(err)
}

func TestLoadNetConf(t *testing.T) {
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

	n, cniVersion, err := loadNetConf(args.StdinData, args.Args)
	te.Nil(err)
	te.Equal(cniVersion, "1.0.0")
	t.Log(n)
}

func TestIPAM(t *testing.T) {
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
	_, addr, err := net.ParseCIDR("10.1.1.6/28")
	te.Nil(err)

	// Assume this is the ipam result, we have current ip net 10.1.1.6/28
	// and gateway ip = 10.1.1.1
	ipamRes := current.Result{
		CNIVersion: "1.0.0",
		Interfaces: []*current.Interface{},
		IPs: []*current.IPConfig{
			{
				Interface: nil,
				Address:   *addr,
				Gateway:   net.ParseIP("10.1.1.1"),
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

	// append veth device info to result's interfaces
	result.Interfaces = append(result.Interfaces, &current.Interface{
		Name:    "",
		Mac:     "abcdef",
		Sandbox: "",
	})
}

// Test from step 2 - step 6
func TestHostSetup(t *testing.T) {
	te := assert.New(t)

	// 2. after ipam, create a veth pair, veth_host and veth_net as gateway pair
	gatewaypair, netpair, err := createHostVethPair()
	te.Nil(err)

	// setup netns, assume it is 'ns1'
	netns, err := ns.GetNS("/var/run/netns/ns1")
	te.Nil(err)
	defer netns.Close()

	// setup these devices
	err = setupHostVethPair(gatewaypair, netpair)
	te.Nil(err)

	// cidr /32 means only one address in this network
	// special ip for gateway
	// result.IPS contains both address & gateway
	// gatewayIP is like: '10.1.1.1/32'
	// IPConfig
	gatewayIP, err := setIPIntoHostPair("10.1.1.1", gatewaypair)
	te.Nil(err)
	te.Equal(gatewayIP, "10.1.1.1/32")

	err = removeHostVethPair("veth_host")
	te.Nil(err)
}
