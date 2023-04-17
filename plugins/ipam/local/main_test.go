package main

import(
	"fmt"
	"testing"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/plugins/pkg/testutils"
)

const (
	ifname string = "eth0"
	nspath string = "/var/run/netns"
)

func TestIPAMLoop(t *testing.T) {
	// 模拟插件被add了20次的情况
	for i := 0; i < 5; i++ {
		conf := fmt.Sprintf(`{
			"cniVersion": "%s",
			"name": "mynet",
			"type": "abcde",
			"dns": {
				"nameservers": ["8.8.8.8"]
			},
			"dataDir": ""
		}`, "1.0.0")

		// Command Line Args for container info
		args := &skel.CmdArgs{
			ContainerID: fmt.Sprintf("dummy-%d", i),
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

		// Release the IP
		err = testutils.CmdDelWithArgs(args, func() error {
			return cmdDel(args)
		})
		if err != nil {
			t.Error("Cmd Del Failed: ", err)
		}
	}
}

