package bpfmap

import (
	"unsafe"

	"github.com/cilium/ebpf"
)

const (
	// change the path to tc global(because bpf program is attached to `tc`)
	LXC_MAP_DEFAULT_PATH = "/sys/fs/bpf/tc/globals/lxc_map"
	LXC_MAP_NAME         = "lxc_map"
	MAX_ENTRIES          = 32
	ETH_ALEN             = 6

	// podmap 实际上没用到 可以删除
	POD_IP_MAP_PATH        = "/sys/fs/bpf/pod_map"
	POD_IP_MAP_NAME        = "pod_map"
	POD_IP_MAP_MAX_ENTRIES = 32

	// vxlan map存储了 本地的vxlan设备号
	VXLAN_MAP_DEFAULT_PATH = "/sys/fs/bpf/tc/globals/node_vxlan_map"
	VXLAN_MAP_NAME         = "node_vxlan_map"
	VXLAN_MAP_MAX_ENTRIES  = 8

	// node cidr map 存储了 某个ip子网地址 所属的node是哪个 用于给vxlan设备查找
	// 例如 10.244.0.0/24的子网地址就属于 master节点
	// 10.244.0.0 === master(10.176.35.14)
	NODE_CIDR_MAP_PATH        = "/sys/fs/bpf/tc/globals/node_cidr_map"
	NODE_CIDR_MAP_NAME        = "node_cidr_map"
	NODE_CIDR_MAP_MAX_ENTRIES = 256

	MODE_VXLAN = 1
)

// keysize = 4bytes, used for pods inside node redirection
type EndpointMapKey struct {
	IP uint32
}

// 4+4+8+8 = 24bytes
type EndpointMapInfo struct {
	LXCIfIndex uint32 // linux container's interface ifindex
	PodIfIndex uint32 // current device's ifindex

	LXCVethMAC [8]byte // Node MAC string
	PodVethMAC [8]byte // MAC string
}

// Not used..
type PodInfoKey struct {
	PodName [8]byte
}

type PodInfoValue struct {
	IP uint32
}

// this is for mapping between net_type -> net_dev_ifindex
type VirtualNetKey struct {
	NetType uint32
}

type VirtualNetValue struct {
	IfIndex uint32
}

// 给定一个pod的ip 去掉最后不同位数的结果得到的cidr值 作为索引查询归属的node
type NodeCIDRKey struct {
	PodIPCIDR uint32
}

// node的真实ip
type NodeCIDRValue struct {
	RealIP uint32
}

// Create a linux-container-map, for current host
//
// Storing pod's ip and their net device's ifindex relations
func CreateLxcMap() (*ebpf.Map, error) {
	const (
		pinPath    = LXC_MAP_DEFAULT_PATH
		name       = LXC_MAP_NAME
		_type      = ebpf.Hash
		keySize    = uint32(unsafe.Sizeof(EndpointMapKey{}))
		valueSize  = uint32(unsafe.Sizeof(EndpointMapInfo{}))
		maxEntries = MAX_ENTRIES
		flags      = 0
	)

	mp, err := CreatePinMapOnce(
		pinPath,
		name,
		_type,
		keySize,
		valueSize,
		maxEntries,
		flags,
	)

	if err != nil {
		return nil, err
	}
	return mp, nil
}

// Create podname - ip mapping
func CreatePodIPMap() (*ebpf.Map, error) {
	const (
		pinPath    = POD_IP_MAP_PATH
		name       = POD_IP_MAP_NAME
		_type      = ebpf.Hash
		keySize    = uint32(unsafe.Sizeof(PodInfoKey{}))
		valueSize  = uint32(unsafe.Sizeof(PodInfoValue{}))
		maxEntries = POD_IP_MAP_MAX_ENTRIES
		flags      = 0
	)

	mp, err := CreatePinMapOnce(
		pinPath,
		name,
		_type,
		keySize,
		valueSize,
		maxEntries,
		flags,
	)

	if err != nil {
		return nil, err
	}
	return mp, nil
}

// create map record of vxlan device id
func CreateVxlanMap() (*ebpf.Map, error) {
	const (
		pinPath    = VXLAN_MAP_DEFAULT_PATH
		name       = VXLAN_MAP_NAME
		_type      = ebpf.Hash
		keySize    = uint32(unsafe.Sizeof(VirtualNetKey{}))
		valueSize  = uint32(unsafe.Sizeof(VirtualNetValue{}))
		maxEntries = VXLAN_MAP_MAX_ENTRIES
		flags      = 0
	)

	mp, err := CreatePinMapOnce(
		pinPath,
		name,
		_type,
		keySize,
		valueSize,
		maxEntries,
		flags,
	)

	if err != nil {
		return nil, err
	}
	return mp, nil
}

func CreateNodeCIDRMap() (*ebpf.Map, error) {
	const (
		pinPath    = NODE_CIDR_MAP_PATH
		name       = NODE_CIDR_MAP_NAME
		_type      = ebpf.Hash
		keySize    = uint32(unsafe.Sizeof(VirtualNetKey{}))
		valueSize  = uint32(unsafe.Sizeof(VirtualNetValue{}))
		maxEntries = NODE_CIDR_MAP_MAX_ENTRIES
		flags      = 0
	)

	mp, err := CreatePinMapOnce(
		pinPath,
		name,
		_type,
		keySize,
		valueSize,
		maxEntries,
		flags,
	)

	if err != nil {
		return nil, err
	}
	return mp, nil
}

// set endpoint value into linux-container-map
func SetLxcMap(key EndpointMapKey, value EndpointMapInfo) error {
	mp, err := GetMapByPinnedPath(LXC_MAP_DEFAULT_PATH)
	if err != nil {
		return err
	}

	return mp.Put(key, value)
}

// set endpoint value into not used map
func SetPodIPMap(key PodInfoKey, value PodInfoValue) error {
	mp, err := GetMapByPinnedPath(POD_IP_MAP_PATH)
	if err != nil {
		return err
	}
	return mp.Put(key, value)
}

// set endpoint value into vxlan dev map
func SetVxlanMap(key VirtualNetKey, value VirtualNetValue) error {
	mp, err := GetMapByPinnedPath(VXLAN_MAP_DEFAULT_PATH)
	if err != nil {
		return err
	}
	return mp.Put(key, value)
}

// set endpoint value into node-cidr map
func SetNodeCIDRMap(key VirtualNetKey, value VirtualNetValue) error {
	mp, err := GetMapByPinnedPath(NODE_CIDR_MAP_PATH)
	if err != nil {
		return err
	}
	return mp.Put(key, value)
}

// Reset the current linux container map
func ResetLxcMap() (int, error) {
	mp, err := GetMapByPinnedPath(LXC_MAP_DEFAULT_PATH)
	if err != nil {
		return -1, err
	}

	iter := mp.Iterate()
	keys := []EndpointMapKey{}
	var key EndpointMapKey
	var value EndpointMapInfo

	for iter.Next(&key, &value) {
		keys = append(keys, key)
	}
	return BatchDelKeyLxcMap(keys)
}

// Reset the current virtual netdev map
func ResetVxlanMap() (int, error) {
	mp, err := GetMapByPinnedPath(VXLAN_MAP_DEFAULT_PATH)
	if err != nil {
		return -1, err
	}

	iter := mp.Iterate()
	keys := []EndpointMapKey{}
	var key EndpointMapKey
	var value EndpointMapInfo

	for iter.Next(&key, &value) {
		keys = append(keys, key)
	}
	return BatchDelKeyLxcMap(keys)
}

// Reset the current linux container map
func ResetNodeCIDRMap() (int, error) {
	mp, err := GetMapByPinnedPath(NODE_CIDR_MAP_PATH)
	if err != nil {
		return -1, err
	}

	iter := mp.Iterate()
	keys := []EndpointMapKey{}
	var key EndpointMapKey
	var value EndpointMapInfo

	for iter.Next(&key, &value) {
		keys = append(keys, key)
	}
	return BatchDelKeyLxcMap(keys)
}

// delete value from linux-container-map
func DelKeyLxcMap(key EndpointMapKey) error {
	mp, err := GetMapByPinnedPath(LXC_MAP_DEFAULT_PATH)
	if err != nil {
		return err
	}
	return mp.Delete(key)
}

func BatchDelKeyLxcMap(keys []EndpointMapKey) (int, error) {
	mp, err := GetMapByPinnedPath(LXC_MAP_DEFAULT_PATH)
	if err != nil {
		return -1, err
	}

	return mp.BatchDelete(keys, &ebpf.BatchOptions{})
}

// given key(ipv4 address), lookup bpfmap
//
// return its endpoint info if exists, else return KeyNotExist error
func GetKeyValueFromLxcMap(key EndpointMapKey) (*EndpointMapInfo, error) {
	mp, err := GetMapByPinnedPath(LXC_MAP_DEFAULT_PATH)
	if err != nil {
		return nil, err
	}

	// Lookup endpoint inside map
	res := &EndpointMapInfo{}
	// return err if keyNotExist, else store value into 'res'
	err = mp.Lookup(key, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func GetKeyValueFromPodIPMap(key PodInfoKey) (*PodInfoValue, error) {
	mp, err := GetMapByPinnedPath(POD_IP_MAP_PATH)
	if err != nil {
		return nil, err
	}

	// Lookup endpoint inside map
	res := &PodInfoValue{}
	// return err if keyNotExist, else store value into 'res'
	err = mp.Lookup(key, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// 从ip - nodereal ip的映射中拿到实际的nodeip做转发
func GetKeyValueFromNodeCIDRMap(key PodInfoKey) (*NodeCIDRValue, error) {
	mp, err := GetMapByPinnedPath(NODE_CIDR_MAP_PATH)
	if err != nil {
		return nil, err
	}

	// Lookup endpoint inside map
	res := &NodeCIDRValue{}
	// return err if keyNotExist, else store value into 'res'
	err = mp.Lookup(key, res)
	if err != nil {
		return nil, err
	}
	return res, nil
}
