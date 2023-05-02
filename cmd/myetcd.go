package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"mycni/pkg/config"
	"net"
	"strconv"
	"strings"

	curLog "log"

	"os"
	"os/signal"
	"sync"
	"syscall"

	etcd "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/pkg/transport"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	errBitMaskAlreadyExists = errors.New("subnet bitmask already exists")
	errBitMaskUpdateFailed  = errors.New("subnet bitmask could not update")
	errSubnetFileNotExist   = errors.New("subnet.json doesn't exist!")
	errBitMaskGetFailed     = errors.New("subnet bitmask could not get!")
)

type EtcdConfig struct {
	Endpoints []string
	Keyfile   string
	Certfile  string
	CAFile    string
}

type Manager interface {
	// GetNetworkConfig(ctx context.Context) (*Config, error)
	// HandleSubnetFile(path string, config *Config, ipMasq bool, sn ip.IP4Net, ipv6sn ip.IP6Net, mtu int) error
	// AcquireLease(ctx context.Context, attrs *LeaseAttrs) (*Lease, error)
	// RenewLease(ctx context.Context, lease *Lease) error
	// WatchLease(ctx context.Context, sn ip.IP4Net, sn6 ip.IP6Net, receiver chan []LeaseWatchResult) error
	// WatchLeases(ctx context.Context, receiver chan []LeaseWatchResult) error
	// CompleteLease(ctx context.Context, lease *Lease, wg *sync.WaitGroup) error
	// HandleLocalSubnetFile(path string) error
	InitBitMask(ctx context.Context) error
	WatchBitMask(ctx context.Context) error
	RenewBitMask(ctx context.Context, bitmask string) error
	Name() string
}

type etcdNewFunc func(ctx context.Context, c *EtcdConfig) (*etcd.Client, etcd.KV, error)

// 假设操作Etcd相关的注册项是这些
type EtcdManager struct {
	cliNewFunc etcdNewFunc  // 初始化handler
	mux        sync.Mutex   // 锁
	kvApi      etcd.KV      // kvapi
	cli        *etcd.Client // 实际的客户端对象
	etcdConfig *EtcdConfig  // etcd配置
}

func ListNodeFromK8s(ctx context.Context) error {
	apiUrl := ""
	kubeconfigPath := "/root/.kube/config"
	cfg, err := clientcmd.BuildConfigFromFlags(apiUrl, kubeconfigPath)
	if err != nil {
		return err
	}
	c, err := clientset.NewForConfig(cfg)
	fmt.Println("k8s Client ok!")

	nodes, err := c.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, nodes := range nodes.Items {
		// fmt.Print(nodes)
		fmt.Printf("Nodename: %s\n", nodes.Name)
		tmp := nodes.Status.Addresses[0]
		fmt.Println(tmp.Address)
	}
	return nil
	// The kube subnet mgr needs to know the k8s node name that it's running on so it can annotate it.
	// If we're running as a pod then the POD_NAME and POD_NAMESPACE will be populated and can be used to find the node
	// name. Otherwise, the environment variable NODE_NAME can be passed in.
	// sm, err := newKubeSubnetManager(ctx, c, sc, nodeName, prefix, useMultiClusterCidr)
}

func shutdownHandler(ctx context.Context, sigs chan os.Signal, cancel context.CancelFunc) {
	// Wait for the context do be Done or for the signal to come in to shutdown.
	select {
	case <-ctx.Done():
		curLog.Print("Stopping shutdownHandler...")
	case <-sigs:
		// Call cancel on the context to close everything down.
		cancel()
		curLog.Print("shutdownHandler sent cancel signal...")
	}

	// Unregister to get default OS nuke behaviour in case we don't exit cleanly
	signal.Stop(sigs)
}

func newTlsConfig(c *EtcdConfig) (*tls.Config, error) {
	if c.Keyfile == "" || c.Certfile == "" || c.CAFile == "" {
		curLog.Fatal("no certificate provided: connecting to etcd with http. This is insecure")
		return nil, nil
	} else {
		tlsInfo := transport.TLSInfo{
			CertFile:      c.Certfile,
			KeyFile:       c.Keyfile,
			TrustedCAFile: c.CAFile,
		}

		tlsConfig, err := tlsInfo.ClientConfig()
		return tlsConfig, err
	}
}

func newEtcdClient(ctx context.Context, c *EtcdConfig) (*etcd.Client, etcd.KV, error) {
	tlscfg, err := newTlsConfig(c)
	if err != nil {
		return nil, nil, err
	}

	cli, err := etcd.New(etcd.Config{
		Endpoints: c.Endpoints,
		TLS:       tlscfg,
	})
	if err != nil {
		return nil, nil, err
	}
	kv := etcd.NewKV(cli)

	//make sure the Client is closed properly
	go func() {
		<-ctx.Done()
		cli.Close()
	}()
	return cli, kv, nil
}

func newEtcdManager(ctx context.Context) (*EtcdManager, error) {
	c := &EtcdConfig{
		Endpoints: []string{"127.0.0.1:2379", "10.176.35.14:2379"},
		Keyfile:   "/etc/kubernetes/pki/etcd/healthcheck-client.key",
		Certfile:  "/etc/kubernetes/pki/etcd/healthcheck-client.crt",
		CAFile:    "/etc/kubernetes/pki/etcd/ca.crt",
	}
	em := &EtcdManager{
		cliNewFunc: newEtcdClient,
		etcdConfig: c,
	}

	var err error
	em.cli, em.kvApi, err = em.cliNewFunc(ctx, c)
	return em, err
}

func (*EtcdManager) Name() string {
	return "mycnid"
}

// 不存在 => 创建 否则 什么都不做
func (em *EtcdManager) InitBitMask(ctx context.Context) error {
	key := "mycni/subnet_bitmask"
	value := strings.Repeat("0", 256)

	// 这里根据ttl向etcd 申请一个lease租约
	lresp, err := em.cli.Grant(ctx, int64(3))
	if err != nil {
		return err
	}

	// 通过发起一个事务查看之前是否存在这个key
	req := etcd.OpPut(key, string(value), etcd.WithLease(lresp.ID))
	cond := etcd.Compare(etcd.Version(key), "=", 0)

	tresp, err := em.cli.Txn(ctx).If(cond).Then(req).Commit()
	if err != nil {
		_, rerr := em.cli.Revoke(ctx, lresp.ID)
		if rerr != nil {
			curLog.Fatal(rerr)
		}
		return err
	}
	// 如果新建BitMask事务不成功
	if !tresp.Succeeded {
		_, rerr := em.cli.Revoke(ctx, lresp.ID)
		if rerr != nil {
			curLog.Fatal(rerr)
		}
		return errBitMaskAlreadyExists
	}

	return nil
}

func (em *EtcdManager) RenewBitMask(ctx context.Context, bitmask string) error {
	key := "mycni/subnet_bitmask"

	// 这里根据ttl向etcd 申请一个lease租约
	lresp, err := em.cli.Grant(ctx, int64(3))
	if err != nil {
		return err
	}

	// 通过发起一个事务查看之前是否存在这个key
	req := etcd.OpPut(key, bitmask, etcd.WithLease(lresp.ID))
	cond := etcd.Compare(etcd.Version(key), ">", 0)

	tresp, err := em.cli.Txn(ctx).If(cond).Then(req).Commit()
	if err != nil {
		_, rerr := em.cli.Revoke(ctx, lresp.ID)
		if rerr != nil {
			curLog.Fatal(rerr)
		}
		return err
	}
	// 如果更新建BitMask事务不成功
	if !tresp.Succeeded {
		_, rerr := em.cli.Revoke(ctx, lresp.ID)
		if rerr != nil {
			curLog.Fatal(rerr)
		}
		return errBitMaskUpdateFailed
	}

	return nil
}

// 获取当前的bitmask状态
func (em *EtcdManager) WatchBitMask(ctx context.Context) (string, error) {
	key := "mycni/subnet_bitmask"

	// 这里根据ttl向etcd 申请一个lease
	lresp, err := em.cli.Grant(ctx, int64(3))
	if err != nil {
		return "", err
	}

	// 通过发起一个事务查看之前是否存在这个key
	req := etcd.OpGet(key, etcd.WithLease(lresp.ID))
	cond := etcd.Compare(etcd.Version(key), ">", 0)
	tresp, err := em.cli.Txn(ctx).If(cond).Then(req).Commit()
	if err != nil {
		_, rerr := em.cli.Revoke(ctx, lresp.ID)
		if rerr != nil {
			curLog.Fatal(rerr)
		}
		return "", err
	}

	// 如果查询bitmask失败
	if !tresp.Succeeded {
		_, rerr := em.cli.Revoke(ctx, lresp.ID)
		if rerr != nil {
			curLog.Fatal(rerr)
		}
		return "", errBitMaskGetFailed
	}

	// 从tresp中提取相应的结果
	bitmask := "000"
	for _, rp := range tresp.Responses {
		for _, ev := range rp.GetResponseRange().Kvs {
			bitmask = string(ev.Value)
		}
	}
	return bitmask, nil
}

// 获取目前所有k8s节点名称
func (em *EtcdManager) NodeNames(ctx context.Context) ([]string, error) {
	const _minionsNodePrefix = "/registry/minions/"

	nodes, err := em.cli.Get(ctx, _minionsNodePrefix, etcd.WithKeysOnly(), etcd.WithPrefix())

	if err != nil {
		return nil, err
	}

	var res []string
	for _, val := range nodes.Kvs {
		node := strings.Replace(string(val.Value), _minionsNodePrefix, "", 1)
		res = append(res, node)
	}
	return res, nil
}

// 节点名称 == real IP也需要知道
// 有了所有的节点名称 我们就可以更新vxlan map中 网段 == 节点IP的映射关系了

func checkLocalSubnetFileSettings() (int, error) {
	subnetConf, err := config.LoadSubnetConfig()
	if err != nil {
		// 加载失败
		return -1, errSubnetFileNotExist
	}

	curSubnet := subnetConf.Subnet
	ip, _, err := net.ParseCIDR(curSubnet)

	if ip.To4() != nil {
		ipParts := strings.Split(ip.String(), ".")
		bit, err := strconv.Atoi(ipParts[2])
		if err != nil {
			return -1, err
		}
		// curLog.Print(bit)
		return bit, nil
	}
	return -1, fmt.Errorf("IPv6 Address not implemented!")
}

func updateLocalSubnetConfig(notUsed int) error {
	// 对指定的文件夹加锁
	// 根据etcd中存储的bitmask
	// 然后创建相应的subnet.json配置
	// fileLock, err := store.NewFileLock(config.DefaultSubnetFile)
	// if err != nil {
	// 	curLog.Fatal("Cannot create subnet.json file at dir!")
	// }

	// fileLock.Lock()
	// defer fileLock.Unlock()

	conf := &config.SubnetConf{
		Bridge: "mycni0",
		Subnet: fmt.Sprintf("10.244.%d.0/24", notUsed),
	}
	data, err := json.Marshal(conf)
	if err != nil {
		return err
	}

	return os.WriteFile(config.DefaultSubnetFile, data, 0644)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	sm, err := newEtcdManager(ctx)
	if err != nil {
		curLog.Fatal("Failed to create etcd manager!")
	}
	curLog.Printf("Created etcd manager %s", "just-a-test")

	// 注册SIGINT/SIGTERM信号的处理函数 回收资源
	curLog.Print("Installing signal handlers")
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		shutdownHandler(ctx, sigs, cancel)
		wg.Done()
	}()

	curLog.Println(sm.Name())

	for {
		curBit, err := checkLocalSubnetFileSettings()
		// 当前节点上还不存在这个文件
		if err != nil {
			// 其他的error全部丢弃
			if err != errSubnetFileNotExist {
				curLog.Fatal("Invalid subnet file!")
			}

			curLog.Println("subnet.json does not exist! syncing...")

			// 首先查看当前是否已经存在了记录集群状态的bitmask
			err = sm.InitBitMask(ctx)
			if err != nil && err != errBitMaskAlreadyExists {
				curLog.Fatal(err.Error())
			}

			// 寻找第一个没有被使用的位置
			curMask, err := sm.WatchBitMask(ctx)
			notUsed := 0
			for ; notUsed <= 255; notUsed++ {
				if curMask[notUsed] == '0' {
					break
				}
			}

			// 更新mask
			tmp := []byte(curMask)
			tmp[notUsed] = byte('1')

			// 同步变化到etcd
			err = sm.RenewBitMask(ctx, string(tmp))
			if err != nil {
				curLog.Fatal(err.Error())
			}

			// 根据curMask配置本地节点文件
			err = updateLocalSubnetConfig(notUsed)
			if err != nil {
				curLog.Fatal("Failed to create subnet.json!")
			}

			curLog.Println("Init current node pod cidr successfully!")
		}

		if curBit >= 0 && curBit <= 255 {
			curLog.Printf("Checking current config ok, with subnet 10.244.%d.0/24", curBit)
		} else {
			curLog.Fatal("Invalid subnet cidr in subnet.json!")
		}

		nodes, err := sm.NodeNames(ctx)
		if err != nil {
			curLog.Fatal(err)
		}
		for _, node := range nodes {
			curLog.Println(node)
		}

		err = ListNodeFromK8s(ctx)
		if err != nil {
			curLog.Fatal(err)
		}
		break

	}
}
