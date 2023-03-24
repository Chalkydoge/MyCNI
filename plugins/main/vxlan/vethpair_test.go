package vxlan

import (
	"fmt"
	"log"
	"mycni/bpfmap"
	"testing"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

const (
	TEST_GW_IP   = "10.1.3.1"
	TEST_POD_IP1 = "10.1.3.2"
	TEST_POD_IP2 = "10.1.3.3"
)

/**
[POD1 10.1.3.2]                     [POD2 10.1.3.3]


			[ASSUMED-GATEWAY 10.1.3.2]

*/

func setupPodVeth(podNS, podIP string) error {
	// setup netns
	netns, err := ns.GetNS(podNS)
	if err != nil {
		return fmt.Errorf("failed to open netns %s: %v", podNS, err)
	}
	defer netns.Close()

	/* Inside every pod, init the network */
	var podPair, hostPair *netlink.Veth

	// enter pod ns, do the follow things
	err = netns.Do(func(hostNS ns.NetNS) error {
		// create a veth pair, one for pod and one for host
		mtu := 1450
		randomName, err := RandomVethName()
		if err != nil {
			return fmt.Errorf("Cannot generate random veth name because of %v", err)
		}
		hostname := "lxc" + randomName

		// curr-endpoint, other-endpoint, mtu
		// [eth0] ----------------[lxc0x3f]
		podPair, hostPair, err = createVethPair("eth0", hostname, mtu)
		if err != nil {
			return err
		}

		// add host veth pair into kubelet ns
		err = setHostVethIntoHost(hostPair, hostNS)
		if err != nil {
			return err
		}

		_, err = setIPIntoPodPair(podIP, podPair)
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
		err = setFIBTableIntoNS(TEST_GW_IP+"/32", podPair)
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

		// then set arp info for layer 2 traffic
		// Note: gatewayIP is with /mask here! need to remove!
		var _mac string // storing host-veth pair mac
		err = hostNS.Do(func(ns.NetNS) error {
			v, err := netlink.LinkByName(hostPair.Attrs().Name)
			if err != nil {
				return err
			}
			veth := v.(*netlink.Veth)
			mac := veth.LinkAttrs.HardwareAddr
			_mac = mac.String()
			return nil
		})

		if err != nil {
			return err
		}

		// Now we create arp inside pod's ns
		err = CreateARPEntry(TEST_GW_IP, _mac, "eth0")
		if err != nil {
			return err
		}

		// write veth pair info LINUX_CONTAINER_MAP / LXC_MAP_DEFAULT_PATH
		err = setVethPairInfo2LxcMap(hostNS, podIP+"/4", hostPair, podPair)
		if err != nil {
			return err
		}

		return nil
	})

	var _mac string
	err = netns.Do(func(hostNS ns.NetNS) error {
		err := hostNS.Do(func(ns.NetNS) error {
			hostVeth, err := netlink.LinkByName(hostPair.Attrs().Name)
			if err != nil {
				return err
			}
			// here should be inside host
			hostPair := hostVeth.(*netlink.Veth)
			// check this mac
			log.Print(hostPair.LinkAttrs.HardwareAddr.String())
			_mac = hostPair.LinkAttrs.HardwareAddr.String()
			return nil
		})
		if err != nil {
			return err
		}
		// here is inside pod
		return CreateARPEntry(TEST_GW_IP, _mac, "eth0")
	})

	return err
}

func TestSetARP(t *testing.T) {
	netns, err := ns.GetNS("/var/run/netns/ns2")
	if err != nil {
		t.Errorf("failed to open netns ns1: %v", err)
	}
	defer netns.Close()

	var _mac string
	err = netns.Do(func(hostNS ns.NetNS) error {
		err := hostNS.Do(func(ns.NetNS) error {
			hostVeth, err := netlink.LinkByName("lxc52fdfc07")
			if err != nil {
				return err
			}
			// here should be inside host
			hostPair := hostVeth.(*netlink.Veth)
			// check this mac
			log.Print(hostPair.LinkAttrs.HardwareAddr.String())
			_mac = hostPair.LinkAttrs.HardwareAddr.String()
			return nil
		})
		if err != nil {
			return err
		}
		// here is inside pod
		return CreateARPEntry(TEST_GW_IP, _mac, "eth0")
	})
	if err != nil {
		t.Error(err)
	}
}

func TestVethPairBPF(t *testing.T) {
	// Now we have ns1 setup complete
	// Just set one namespace
	err := setupPodVeth("/var/run/netns/ns2", TEST_POD_IP2)
	if err != nil {
		t.Error(err)
	}
}

func TestBPFMap(t *testing.T) {
	mp, err := bpfmap.CreateLxcMap()
	if err != nil {
		t.Error(err)
	}

	epInfo := &bpfmap.EndpointMapInfo{}
	mp.Lookup(bpfmap.EndpointMapKey{IP: InetIpToUInt32(TEST_POD_IP2)}, epInfo)

	// Assumed to be fe:40:4b:94:22:ce
	t.Log(epInfo.LXCVethMAC)

	// Assumed to be 5a:72:11:d8:c5:69
	t.Log(epInfo.PodVethMAC)
}

// func TestAttachBPF2Ingress(t *testing.T) {

// }

// ip netns exec ns2 arp -d 10.1.3.1
// ip netns exec ns2 ip route del 10.1.3.1 dev eth0 scope link
// ip netns exec ns2 ip route del default via 10.1.3.1 dev eth0
// ip netns exec ns2 ip link del eth0
