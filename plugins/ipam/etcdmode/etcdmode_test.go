package main

import(
	"fmt"
	"testing"
	"mycni/etcdwrap"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/plugins/pkg/testutils"
)

const (
	ifname string = "eth0"
	nspath string = "/some/where"
)

func TestCmdAdd(t *testing.T) {
	// Just for loop breaked with errors
	// etcdwrap.Init()
	conf := fmt.Sprintf(`{
		"cniVersion": "%s",
		"name": "mynet",
		"type": "abcde",
		"master": "foo0",
		"ipam": {
			"type": "etcdmode"
		}
	}`, "1.0.0")

	// Command Line Args for container info
	args := &skel.CmdArgs{
		ContainerID: "dummy",
		Netns:       nspath,
		IfName:      ifname,
		StdinData:   []byte(conf),
	}

	// Allocate the IP
	r, _, err := testutils.CmdAddWithArgs(args, func() error {
		return cmdAdd(args)
	})
	if err != nil {
		t.Error("Cmd Add Failed: ", err)
	}
	t.Log(r)

	// Print the result
	// Now we have assigned valid ip for device, but no layer2 interfaces set
	// {1.0.0 [] [{Interface:<nil> Address:{IP:10.1.1.4 Mask:fffffff0} Gateway:10.1.1.1}] [] {[]  [] []}}
}

func TestCmdCheck(t *testing.T) {
	conf := fmt.Sprintf(`{
		"cniVersion": "%s",
		"name": "mynet",
		"type": "abcde",
		"master": "foo0",
		"ipam": {
			"type": "etcdmode"
		}
	}`, "1.0.0")

	// Command Line Args for container info
	args := &skel.CmdArgs{
		ContainerID: "dummy",
		Netns:       nspath,
		IfName:      ifname,
		StdinData:   []byte(conf),
	}

	// Allocate the IP
	err := testutils.CmdCheckWithArgs(args, func() error {
		return cmdCheck(args)
	})
	if err != nil {
		t.Error("Cmd Check Failed: ", err)
	}
}

func TestCmdDel(t *testing.T) {
	cli, err := etcdwrap.GetEtcdClient()
	defer cli.CloseEtcdClient()
	if err != nil {
		t.Error("Get Etcd Client failed! ", err)
	}

	conf := fmt.Sprintf(`{
		"cniVersion": "%s",
		"name": "mynet",
		"type": "abcde",
		"master": "foo0",
		"ipam": {
			"type": "etcdmode"
		}
	}`, "1.0.0")

	// Command Line Args for container info
	args := &skel.CmdArgs{
		ContainerID: "dummy",
		Netns:       nspath,
		IfName:      ifname,
		StdinData:   []byte(conf),
	}

	// Allocate the IP
	err = testutils.CmdDelWithArgs(args, func() error {
		return cmdDel(args)
	})
	if err != nil {
		t.Error("Cmd Del Failed: ", err)
	}
}