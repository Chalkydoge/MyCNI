package bpfmap

import (
	"mycni/utils"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/rlimit"
)

// find bpfmap file by pinPath
func GetMapByPinnedPath(pinPath string, opts ...*ebpf.LoadPinOptions) (*ebpf.Map, error) {
	var options *ebpf.LoadPinOptions
	if len(opts) == 0 {
		options = &ebpf.LoadPinOptions{}
	} else {
		options = opts[0]
	}

	mp, err := ebpf.LoadPinnedMap(pinPath, options)
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
	if err := rlimit.RemoveMemlock(); err != nil {
		return nil, err
	}

	mp, err := ebpf.NewMap(&spec)
	if err != nil {
		return nil, err
	}
	return mp, nil
}

// Only create a pinned map once, if multiple time called, will not new maps
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
