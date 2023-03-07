package allocator

import (
	"testing"
	"github.com/stretchr/testify/assert"

	"mycni/etcdwrap"
	"mycni/plugins/ipam/etcdmode/initpool"
)

func TestAllocateIP2Host(t *testing.T) {
	var r bool
	var expect_ip string
	te := assert.New(t)
	
	// init etcd client
	etcdwrap.Init()
	cli, err := etcdwrap.GetEtcdClient()
	te.Nil(err)
	te.NotNil(cli)
	te.Equal(cli.GetInitPoolStatus(), false)

	// then init ip pool
	r, err = initpool.InitPool(cli)
	te.Nil(err)
	te.Equal(r, true)
	te.Equal(cli.GetInitPoolStatus(), true)

	// then try to get one IP
	expect_ip, err = AllocateIP2Host(cli)
	te.Nil(err)
	// te.Equal(expect_ip, )	
	t.Log("Allocate IP " + expect_ip + " to master")
	
	// double check that ip is ok
	var res string
	res, err = cli.GetKV("mycni/ipam/master")
	te.Nil(err)
	te.Equal(res, expect_ip)

	// Show whether gateway is assigned
	res, err = cli.GetKV("mycni/ipam/master/gateway")
	te.Nil(err)
	t.Log("Expected gateway IP " + res + " of master node")

	// try to restore back
	r, err = ReleaseHostIP(cli)
	te.Nil(err)
	te.Equal(r, true)

	// double check that ip is released
	res, err = cli.GetKV("mycni/ipam/master")
	te.Nil(err)
	te.Equal(res, "")

	// double check that gateway ip is released
	res, err = cli.GetKV("mycni/ipam/master/gateway")
	te.Nil(err)
	te.Equal(res, "")

	// then release ip pool?
	r, err = initpool.ReleasePool(cli)
	te.Nil(err)
	te.Equal(r, true)

	// double check that ippool is released
	res, err = cli.GetKV("mycni/ipam/pool")
	te.Nil(err)
	te.Equal(res, "")

	// finally release client
	cli.CloseEtcdClient()
	te.Nil(err)
}

// func TestPool(t *testing.T) {
// 	te := assert.New(t)

// 	etcdwrap.Init()
// 	cli, err := etcdwrap.GetEtcdClient()
// 	te.Nil(err)
// 	te.NotNil(cli)
// 	te.Equal(cli.GetInitPoolStatus(), false)

// 	var r1, r2 bool
// 	r1, err = InitPool(cli)
// 	te.Nil(err)
// 	te.Equal(r1, true)

// 	r2, err = ReleasePool(cli)
// 	te.Nil(err)
// 	te.Equal(r2, true)	
// }