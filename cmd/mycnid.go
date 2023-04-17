package main

import (
	"context"
	"flag"
	"fmt"
	mycniconfig "mycni/pkg/config"
	"net"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	appName = "testcnid"
)

var (
	log = logf.Log.WithName(appName)
)

type DaemonConf struct {
	nodeName string
	podCIDR  string
}

func (conf *DaemonConf) addFlags() {
	flag.StringVar(&conf.nodeName, "node", "", "current node name")
	flag.StringVar(&conf.podCIDR, "cluster-cidr", "", "current node's pod cidr")
}

func (conf *DaemonConf) parseConfig() error {
	if _, _, err := net.ParseCIDR(conf.podCIDR); err != nil {
		return fmt.Errorf("Pod CIDR is invaild: %v", err)
	}

	if len(conf.nodeName) == 0 {
		conf.nodeName = os.Getenv("NODE_NAME")
	}
	if len(conf.nodeName) == 0 {
		return fmt.Errorf("node name is empty")
	}
	return nil
}

// 每个节点上运行的Daemon进程 用于同步多个节点之间的路由信息
// 多个节点内部节点的IPNet分配情况
func RunController(conf *DaemonConf) error {
	mgr, err := manager.New(config.GetConfigOrDie(), manager.Options{})
	log.Info("OK")
	return nil

	if err != nil {
		log.Error(err, "could not create manager")
		return err
	}

	reconciler, err := NewReconciler(conf, mgr)
	if err != nil {
		return err
	}
	log.Info("create reconciler success")
	err = builder.
		ControllerManagedBy(mgr).
		For(&corev1.Node{}).
		WithEventFilter(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				old, ok := e.ObjectOld.(*corev1.Node)
				if !ok {
					return true
				}
				new, ok := e.ObjectNew.(*corev1.Node)
				if !ok {
					return true
				}
				return old.Spec.PodCIDR != new.Spec.PodCIDR
			},
		}).
		Complete(reconciler)
	if err != nil {
		log.Error(err, "could not create controller")
		return err
	}

	return mgr.Start(signals.SetupSignalHandler())
}

func main() {
	logf.SetLogger(zap.New())

	conf := &DaemonConf{}
	conf.addFlags()

	flag.Parse()

	if err := conf.parseConfig(); err != nil {
		log.Error(err, "failed to parse config")
		os.Exit(1)
	}

	if err := RunController(conf); err != nil {
		log.Error(err, "failed to run controller")
		os.Exit(1)
	}
}

type Reconciler struct {
	client      client.Client
	clusterCIDR *net.IPNet

	// hostLink     netlink.Link
	// routes       map[string]netlink.Route
	config       *DaemonConf
	subnetConfig *mycniconfig.SubnetConf
}

func NewReconciler(conf *DaemonConf, mgr manager.Manager) (*Reconciler, error) {
	// 从conf内读取的podCIDR
	// conf: subnet "10.244.0.0/8"
	// "10.244.1.0/8", ..., "10.244.255.0/8"
	_, cidr, err := net.ParseCIDR(conf.podCIDR)
	if err != nil {
		return nil, err
	}

	node := &corev1.Node{}
	if err := mgr.GetAPIReader().Get(context.TODO(), types.NamespacedName{Name: conf.nodeName}, node); err != nil {
		return nil, err
	}

	hostIP, err := getNodeInternalIP(node)
	if err != nil {
		return nil, fmt.Errorf("failed to get host ip for node %s", conf.nodeName)
	}

	log.Info("get nodeinfo", "host ip", hostIP.String(), "current node's pod cidr", cidr.String())
	return nil, nil
}

func getNodeInternalIP(node *corev1.Node) (net.IP, error) {
	if node == nil {
		return nil, fmt.Errorf("empty node")
	}

	var ip net.IP
	for _, addr := range node.Status.Addresses {
		if addr.Type == corev1.NodeInternalIP {
			ip = net.ParseIP(addr.Address)
			break
		}
	}

	if len(ip) == 0 {
		return nil, fmt.Errorf("node %s ip is nil", node.Name)
	}

	return ip, nil
}

func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log.Info("start reconcile", "key", req.NamespacedName.Name)
	result := reconcile.Result{}
	nodes := &corev1.NodeList{}
	// 向k8s集群apiserver 获取当前集群内的节点信息
	if err := r.client.List(ctx, nodes); err != nil {
		return result, err
	}

	// cidrs := make(map[string]netlink.Route)
	for _, node := range nodes.Items {
		log.Info("Iterating over node %s, with api version %s", node.APIVersion, node.Name)
	}
	return result, nil
}
