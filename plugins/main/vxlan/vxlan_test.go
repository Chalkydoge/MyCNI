package vxlan

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

// Snippets for generating a JSON network configuration string.
const (
	netConfStr = `
	"cniVersion": "%s",
	"name": "testConfig",
	"type": "bridge",
	"bridge": "%s"`

	vlan = `,
	"vlan": %d`

	netDefault = `,
	"isDefaultGateway": true`

	ipamStartStr = `,
    "ipam": {
        "type":    "host-local"`

	ipamDataDirStr = `,
        "dataDir": "%s"`

	ipamResolvConfStr = `,
		"resolvConf": "%s"`

	ipMasqConfStr = `,
	"ipMasq": %t`

	// Single subnet configuration (legacy)
	subnetConfStr = `,
        "subnet":  "%s"`
	gatewayConfStr = `,
        "gateway": "%s"`

	// Ranges (multiple subnets) configuration
	rangesStartStr = `,
        "ranges": [`
	rangeSubnetConfStr = `
            [{
                "subnet":  "%s"
            }]`
	rangeSubnetGWConfStr = `
            [{
                "subnet":  "%s",
                "gateway": "%s"
            }]`
	rangesEndStr = `
        ]`

	ipamEndStr = `
    }`

	macspoofchkFormat = `,
        "macspoofchk": %t`

	argsFormat = `,
    "args": {
        "cni": {
            "mac": %q
        }
    }`

	runtimeConfig = `,
    "RuntimeConfig": {
        "mac": %q
    }`
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


func TestVxlan(t *testing.T) {
	test := assert.New(t)

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
		Args:        "K8S_POD_INFRA_CONTAINER_ID=308102901b7fe9538fcfc71669d505bc09f9def5eb05adeddb73a948bb4b2c8b;K8S_POD_UID=d392609d-6aa2-4757-9745-b85d35e3d326;IgnoreUnknown=1;K8S_POD_NAMESPACE=kube-system;K8S_POD_NAME=coredns-c676cc86f-4kz2t",
		Path:        "/opt/cni/bin",
		StdinData:   []byte(conf),
	}

	err := cmdAdd(args)
	te.Nil(err)
}