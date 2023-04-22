package bpfmap

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateLXCMap(t *testing.T) {
	// first create a pinned map(shared for all prog on this host)
	test := assert.New(t)
	mp, err := CreateLxcMap()
	test.Nil(err)
	test.NotNil(mp)

	err = mp.Put(EndpointMapKey{IP: 1}, EndpointMapInfo{
		PodIfIndex: 12,
		LXCIfIndex: 13,
		PodVethMAC: [8]byte{0x3f, 0x3f, 0x3f, 0x3f, 0x3f, 0x3f, 0x0, 0x0},
		LXCVethMAC: [8]byte{0xab, 0xcd, 0xef, 0xab, 0xcd, 0xef, 0x0, 0x0}, // two bytes for padding
	})
	test.Nil(err)

	// err = mp.Put(EndpointMapKey{IP: 2}, EndpointMapInfo{
	// 	PodIfIndex: 14,
	// 	LXCIfIndex: 15,
	// 	PodVethMAC: [6]byte{0x3f, 0x3f, 0x3f, 0x3f, 0x3f, 0x3f},
	// 	LXCVethMAC: [6]byte{0xab, 0xcd, 0xef, 0xab, 0xcd, 0xef},
	// })
	// test.Nil(err)

	epInfo := &EndpointMapInfo{}
	err = mp.Lookup(EndpointMapKey{IP: 1}, epInfo)
	test.Nil(err)
	test.NotNil(epInfo)

	test.Equal(epInfo.PodIfIndex, uint32(12))
	test.Equal(epInfo.LXCIfIndex, uint32(13))

	err = mp.Delete(EndpointMapKey{IP: 1})
	test.Nil(err)

	// err = mp.Delete(EndpointMapKey{IP: 2})
	// test.Nil(err)
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

func TestLookup(t *testing.T) {
	mp, err := CreateLxcMap()
	if err != nil {
		t.Log(err)
	}

	epInfo := &EndpointMapInfo{}
	err = mp.Lookup(EndpointMapKey{IP: InetIpToUInt32("10.1.2.12")}, epInfo)
	if err != nil {
		t.Log(err)
	}
	t.Log(epInfo)
}

func TestResetMap(t *testing.T) {
	code, err := ResetLxcMap()
	if err != nil {
		t.Log(err)
	}
	if code < 0 {
		t.Log("Error happened while deleting keys!")
	}
}

func TestIterate(t *testing.T) {
	mp, err := CreateLxcMap()
	if err != nil {
		t.Log(err)
	}

	iter := mp.Iterate()
	keys := []EndpointMapKey{}
	var key EndpointMapKey
	var value EndpointMapInfo

	for iter.Next(&key, &value) {
		keys = append(keys, key)
	}
	t.Log(keys)
}

func TestVxlanMap(t *testing.T) {
	mp, err := CreateVxlanMap()
	if err != nil {
		t.Log(err)
	}

	key := &VirtualNetKey{
		NetType: MODE_VXLAN,
	}
	val := &VirtualNetValue{
		IfIndex: 114514,
	}
	err = mp.Put(key, val)
	if err != nil {
		t.Log(err)
	}

	// 遍历一下
	iter := mp.Iterate()
	keys := []VirtualNetKey{}
	var tmp VirtualNetKey
	var value VirtualNetValue

	for iter.Next(&tmp, &value) {
		keys = append(keys, tmp)
	}
	t.Log(keys)

	_, err = ResetVxlanMap()
	if err != nil {
		t.Log(err)
	}
}

func TestNodeCIDRMap(t *testing.T) {
	mp, err := CreateNodeCIDRMap()
	if err != nil {
		t.Log(err)
	}

	// 遍历一下
	iter := mp.Iterate()
	keys := []NodeCIDRKey{}
	var tmp NodeCIDRKey
	var value NodeCIDRValue

	for iter.Next(&tmp, &value) {
		keys = append(keys, tmp)
	}
	t.Log(keys)
	t.Log(value)
}

// 给两个节点写好cidr 然后给vxlan设备map写入对应的编号
func TestSetupvxlan(t *testing.T) {
	mp, err := CreateVxlanMap()
	if err != nil {
		t.Log(err)
	}
	key := &VirtualNetKey{
		NetType: MODE_VXLAN,
	}
	val := &VirtualNetValue{
		IfIndex: 1021,
	}
	err = mp.Put(key, val)
	if err != nil {
		t.Log(err)
	}
	t.Log("Vxlan dev map ok!")

	node_mp, err := CreateNodeCIDRMap()
	if err != nil {
		t.Log(err)
	}

	node_key := &NodeCIDRKey{
		PodIPCIDR: InetIpToUInt32("10.244.1.0"),
	}
	node_val := &NodeCIDRValue{
		RealIP: InetIpToUInt32("10.176.35.11"),
	}
	err = node_mp.Put(node_key, node_val)
	if err != nil {
		t.Log(err)
	}

	node_key = &NodeCIDRKey{
		PodIPCIDR: InetIpToUInt32("10.244.0.0"),
	}
	node_val = &NodeCIDRValue{
		RealIP: InetIpToUInt32("10.176.35.14"),
	}
	err = node_mp.Put(node_key, node_val)
	if err != nil {
		t.Log(err)
	}

	t.Log("Node cidr map set complete!")
}

func TestDelvxlan(t *testing.T) {
	_, err := ResetVxlanMap()
	if err != nil {
		t.Log(err)
	}
	t.Log("Vxlan dev empty")

	_, err = ResetNodeCIDRMap()
	if err != nil {
		t.Log(err)
	}
	t.Log("Node cidr map empty")
}

// tc qdisc del dev vethbbde6589 clsact
// tc qdisc add dev vethbbde6589 clsact
// tc filter replace dev vethbbde6589 ingress handle 0x1 bpf da obj veth_ingress.bpf.o
