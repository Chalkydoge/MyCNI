package etcdwrap

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestInitClient(t *testing.T) {
	te := assert.New(t)

	Init()
	cli, err := GetEtcdClient()
	te.Nil(err)
	te.NotNil(cli)
	te.NotNil(cli.client)
	te.Equal(cli.poolInitStatus, false)
}

func TestPutAndGetAndDelValue(t *testing.T) {
	te := assert.New(t)
	Init()

	cli, err := GetEtcdClient()
	te.Nil(err)
	te.NotNil(cli)
	te.NotNil(cli.client)
	te.Equal(cli.poolInitStatus, false)

	err = cli.PutKV("foo1", "bar1")
	te.Nil(err)

	var value string
	value, err = cli.GetKV("foo1")
	te.Nil(err)
	te.NotNil(value)
	te.Equal(value, "bar1")

	err = cli.DelKV("foo1")
	te.Nil(err)

	value, err = cli.GetKV("foo1")
	te.Nil(err)
	te.NotNil(value)
	te.Equal(value, "")

	cli.CloseEtcdClient()
	te.Nil(err)
}