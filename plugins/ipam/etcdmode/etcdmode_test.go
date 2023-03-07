package main

import(
	"github.com/containernetworking/plugins/pkg/testutils"
)

const (
	ifname string = "eth0"
	nspath string = "/some/where"
)

func TestA() {
	// A demo cni plugin conf
	conf := fmt.Sprintf(`{
		"cniVersion": "%s",
		"name": "mynet",
		"type": "ipvlan",
		"master": "foo0",
			"ipam": {
				"type": "etcdmode"
			}
	}`, ver)

	// Command line args of cnio
	args := &skel.CmdArgs{
		ContainerID: "dummy",
		Netns:       nspath,
		IfName:      ifname,
		StdinData:   []byte(conf),
	}
	
	// Allocate the IP
	r, raw, err := testutils.CmdAddWithArgs(args, func() error {
		return cmdAdd(args)
	})
	Expect(err).NotTo(HaveOccurred())
	if testutils.SpecVersionHasIPVersion(ver) {
		Expect(strings.Index(string(raw), "\"version\":")).Should(BeNumerically(">", 0))
	}

	result, err := types100.GetResult(r)
	Expect(err).NotTo(HaveOccurred())
	result, err := types100.GetResult(r)
	Expect(err).NotTo(HaveOccurred())

	// Gomega is cranky about slices with different caps
	Expect(*result.IPs[0]).To(Equal(
		types100.IPConfig{
			Address: mustCIDR("10.1.2.2/24"),
			Gateway: net.ParseIP("10.1.2.1"),
		}))

	Expect(*result.IPs[1]).To(Equal(
		types100.IPConfig{
			Address: mustCIDR("2001:db8:1::2/64"),
			Gateway: net.ParseIP("2001:db8:1::1"),
		},
	))
	Expect(len(result.IPs)).To(Equal(2))
}