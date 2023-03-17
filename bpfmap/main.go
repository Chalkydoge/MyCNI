package bpfmap

import (
	"unsafe"

	"github.com/cilium/ebpf"
)

const (
	LXC_MAP_DEFAULT_PATH = "/sys/fs/bpf"
	MAX_ENTRIES          = 255
)

// keysize = 4bytes
type EndpointMapKey struct {
	IP uint32
}

// 4+4+8+8 = 24bytes
type EndpointMapInfo struct {
	IfIndex    uint32  // current device's ifindex
	LXCIfIndex uint32  // linux container's interface ifindex
	MAC        [8]byte // MAC string
	NodeMAC    [8]byte // Node MAC string
}

// Create a linux-container-map, for current host
//
// Storing pod's ip and their net device's ifindex relations
func CreateLxcMap() (*ebpf.Map, error) {
	const (
		pinPath    = LXC_MAP_DEFAULT_PATH
		name       = "lxc_map"
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

// set endpoint value into linux-container-map
func SetLxcMap(key EndpointMapKey, value EndpointMapInfo) error {
	mp, err := GetMapByPinnedPath(LXC_MAP_DEFAULT_PATH)
	if err != nil {
		return err
	}

	return mp.Put(key, value)
}

// delete value from linux-container-map
func DelKeyLxcMap(key EndpointMapKey) error {
	mp, err := GetMapByPinnedPath(LXC_MAP_DEFAULT_PATH)
	if err != nil {
		return err
	}
	return mp.Delete(key)
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
