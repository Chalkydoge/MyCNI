package etcdwrap

import (
	"context"
	"fmt"
	"mycni/consts"
	"mycni/utils"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/transport"
)

const (
	sampleTimeout = 5 * time.Second
)

type WrappedClient struct {
	client         *clientv3.Client
	poolInitStatus bool
}

var __innerGetEtcdClient func() (*WrappedClient, error)

func GetEtcdClient() (*WrappedClient, error) {
	if __innerGetEtcdClient == nil {
		return nil, nil
	}
	return __innerGetEtcdClient()
}

func _innerGetEtcdClient() func() (*WrappedClient, error) {
	var _cli *WrappedClient

	return func() (*WrappedClient, error) {
		if _cli != nil {
			return _cli, nil
		} else {
			tlsInfo := transport.TLSInfo{
				CertFile:      consts.K8S_CERT_FILEPATH,
				KeyFile:       consts.K8S_KEY_FILEPATH,
				TrustedCAFile: consts.K8S_CA_FILEPATH,
			}

			tlsConfig, err := tlsInfo.ClientConfig()

			cli, err := clientv3.New(clientv3.Config{
				Endpoints:   []string{"127.0.0.1:2379", "10.128.47.22:2379"},
				DialTimeout: sampleTimeout,
				TLS:         tlsConfig,
			})

			if err != nil {
				// handle error
				return nil, fmt.Errorf("Failed to connected etcd server, message: %w", err)
			}

			_cli = &WrappedClient{
				client:         cli,
				poolInitStatus: false,
			}

			return _cli, nil
		}
	}
}

func Init() {
	if __innerGetEtcdClient == nil {
		__innerGetEtcdClient = _innerGetEtcdClient()
	}
}

func (cli *WrappedClient) CloseEtcdClient() {
	cli.client.Close()
}

// Given key, fetch the value from etcd
func (cli *WrappedClient) GetKV(key string) (string, error) {
	resp, err := cli.client.Get(context.TODO(), key)
	if err != nil {
		return "", err
	}

	if len(resp.Kvs) > 0 {
		// return last item's first value
		return string(resp.Kvs[len(resp.Kvs)-1:][0].Value), nil
	}
	return "", nil
}

// update value according to the given key
func (cli *WrappedClient) PutKV(key, value string) error {
	_, err := cli.client.Put(context.TODO(), key, value)

	if err != nil {
		return err
	}

	return nil
}

// delete value according to the given key
func (cli *WrappedClient) DelKV(key string) error {
	_, err := cli.client.Delete(context.TODO(), key)
	if err != nil {
		return err
	}
	return err
}

// set pool status for other packages
func (cli *WrappedClient) SetInitPoolStatus(stat bool) {
	cli.poolInitStatus = stat
}

// get pool status for other packages
func (cli *WrappedClient) GetInitPoolStatus() bool {
	resp, err := cli.GetKV(utils.GetIPPoolPath())
	if err != nil {
		return false
	}
	if resp != "" {
		return true
	}
	return false
}
