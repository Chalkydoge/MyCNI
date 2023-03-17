package bpfmap

import (
	"mycni/utils"

	"github.com/cilium/ebpf"
)

// find bpfmap file by pinPath
func GetMapByPinnedPath(pinPath string, options ...*ebpf.LoadPinOptions) (*ebpf.Map, error) {
	var opts *ebpf.LoadPinOptions
	if len(options) == 0 {
		opts = &ebpf.LoadPinOptions{}
	} else {
		opts = options[0]
	}

	mp, err := ebpf.LoadPinnedMap(pinPath, opts)
	if err != nil {
		return nil, err
	}
	return mp, nil
}

func createMap(name string, _type ebpf.MapType, keySize, valueSize, maxEntries, flags uint32) (*ebpf.Map, error) {
	spec := ebpf.MapSpec{
		Name:       name,
		Type:       ebpf.Hash,
		KeySize:    keySize,
		ValueSize:  valueSize,
		MaxEntries: maxEntries,
		Flags:      flags,
	}

	mp, err := ebpf.NewMap(&spec)
	if err != nil {
		return nil, err
	}
	return mp, nil
}

// only create a pinned map once, if multiple time called, will not new maps
func CreatePinMapOnce(pinPath, name string, _type ebpf.MapType, keySize, valueSize, maxEntries, flags uint32) (*ebpf.Map, error) {
	if utils.PathExists(pinPath) {
		return GetMapByPinnedPath(pinPath)
	}

	mp, err := createMap(
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

	err = mp.Pin(pinPath)
	if err != nil {
		return nil, err
	}
	return mp, nil
}
