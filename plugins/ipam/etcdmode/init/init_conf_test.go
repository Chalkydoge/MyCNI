package init

import (
	"testing"
	"github.com/stretchr/testify/assert"

	"mycni/etcdwrap"
)

func TestRandomGenerateIpCIDRs(t *testing.T) {
	te := assert.New(t)

	res := RandomGenerateIpCIDRs()
	// t.Log(res)
	te.Equal(len(res), 16)
}

func TestPool(t *testing.T) {
	te := assert.New(t)

	etcdwrap.Init()
	cli, err := etcdwrap.GetEtcdClient()
	te.Nil(err)
	te.NotNil(cli)
	te.Equal(cli.GetInitPoolStatus(), false)

	var r1, r2 bool
	r1, err = InitPool(cli)
	te.Nil(err)
	te.Equal(r1, true)

	r2, err = ReleasePool(cli)
	te.Nil(err)
	te.Equal(r2, true)	
}