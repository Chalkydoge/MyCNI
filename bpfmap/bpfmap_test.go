package bpfmap

import (
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
