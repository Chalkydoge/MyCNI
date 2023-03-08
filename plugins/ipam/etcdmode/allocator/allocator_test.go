package allocator

import (
	"testing"
	"github.com/stretchr/testify/assert"

	"mycni/etcdwrap"
	"mycni/plugins/ipam/etcdmode/initpool"
	current "github.com/containernetworking/cni/pkg/types/100"
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
}

func TestAllocateIP2Pod(t *testing.T) {
	// we have two container, with both 'eth0' in its ns
	containerID := "dummy0"
	containerID1 := "dummy1"
	ifname := "eth0"

	te := assert.New(t)

	// init etcd client
	etcdwrap.Init()
	cli, err := etcdwrap.GetEtcdClient()
	te.Nil(err)
	te.NotNil(cli)
	// te.Equal(cli.GetInitPoolStatus(), false)

	// Allocate 2 devices here
	var ipConf *current.IPConfig
	ipConf, err = AllocateIP2Pod(containerID, ifname, cli)
	te.Nil(err)
	t.Log(ipConf)

	ipConf, err = AllocateIP2Pod(containerID1, ifname, cli)
	te.Nil(err)
	t.Log(ipConf)

	// try to release ip back
	var releasePodRes bool
	releasePodRes, err = ReleasePodIP(containerID, ifname, cli)
	te.Nil(err)
	te.Equal(releasePodRes, true)

	releasePodRes, err = ReleasePodIP(containerID1, ifname, cli)
	te.Nil(err)
	te.Equal(releasePodRes, true)

	// try to restore back
	var releaseHostRes bool 
	releaseHostRes, err = ReleaseHostIP(cli)
	te.Nil(err)
	te.Equal(releaseHostRes, true)

	// double check that ip is released
	var releasedIP string
	releasedIP, err = cli.GetKV("mycni/ipam/master")
	te.Nil(err)
	te.Equal(releasedIP, "")

	// double check that gateway ip is released
	releasedIP, err = cli.GetKV("mycni/ipam/master/gateway")
	te.Nil(err)
	te.Equal(releasedIP, "")

	// finally release client
	cli.CloseEtcdClient()
	te.Nil(err)
}