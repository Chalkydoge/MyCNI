package main

import (
	"fmt"
	"mycni/pkg/testutils"
	"testing"

	"github.com/containernetworking/cni/pkg/skel"
)

func TestCmdAdd(t *testing.T) {
	for i := 1; i < 2; i++ {
		conf := fmt.Sprintf(`{
			"cniVersion": "%s",
			"name": "mynet",
			"type": "vxlan",
			"ipam": {
				"type": "local",
				"dns": {
					"nameservers": ["8.8.8.8"]
				}
			},
			"dataDir": ""
		}`, "1.0.0")

		args := &skel.CmdArgs{
			ContainerID: "308102901b7fe9538fcfc71669d505bc09f9def5eb05adeddb73a948bb4b2c8b",
			Netns:       fmt.Sprintf("/var/run/netns/ns%d", i),
			IfName:      "eth0",
			Args:        fmt.Sprintf("K8S_POD_NAMESPACE=test_only_%d;K8S_POD_NAME=foobar-c676cc86f-4kz2t", i),
			Path:        "/usr/local/bin",
			StdinData:   []byte(conf),
		}
		_, _, err := testutils.CmdAddWithArgs(args, func() error {
			return cmdAdd(args)
		})
		if err != nil {
			t.Log(err)
		}
		t.Logf("Setting up pod %d", i)
	}
}

func TestIPAMDelegate2(t *testing.T) {
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
		Netns:       "/var/run/netns/ns2",
		IfName:      "eth0",
		Args:        "K8S_POD_NAMESPACE=test_only;K8S_POD_NAME=foobar-c676cc86f-334ff",
		Path:        "/home/ubuntu/go/src/mycni/bin",
		StdinData:   []byte(conf),
	}
	_, _, err := testutils.CmdAddWithArgs(args, func() error {
		return cmdAdd(args)
	})

	if err != nil {
		t.Log(err)
	}
}

func TestCmdDel(t *testing.T) {
	for i := 1; i < 3; i++ {
		conf := fmt.Sprintf(`{
			"cniVersion": "%s",
			"name": "mynet",
			"type": "vxlan",
			"ipam": {
				"type": "local",
				"dns": {
					"nameservers": ["8.8.8.8"]
				}
			},
			"dataDir": ""
		}`, "1.0.0")

		args := &skel.CmdArgs{
			ContainerID: "308102901b7fe9538fcfc71669d505bc09f9def5eb05adeddb73a948bb4b2c8b",
			Netns:       fmt.Sprintf("/var/run/netns/ns%d", i),
			IfName:      "eth0",
			Args:        fmt.Sprintf("K8S_POD_NAMESPACE=test_only_%d;K8S_POD_NAME=foobar-c676cc86f-4kz2t", i),
			Path:        "/home/ubuntu/go/src/mycni/bin",
			StdinData:   []byte(conf),
		}

		err := testutils.CmdDelWithArgs(args, func() error {
			return cmdDel(args)
		})
		if err != nil {
			t.Log(err)
		}
		t.Logf("Removing pod %d ok", i)
	}
}

func TestParsing(t *testing.T) {
	ips := "10.1.0.11"
	tmp := InetIpToUInt32(ips)
	t.Logf("uint32 of ip %s is %d", ips, tmp)

	tmp_ip := UInt32ToInetIP(tmp)
	t.Logf("re-parsing ip is %s", tmp_ip)
}
