package main

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	resyncPeriod = 5 * time.Minute
)

type kubeSubnetManager struct {
	enableIPv4 bool
	// enableIPv6                bool
	// annotations               annotations
	client         clientset.Interface
	nodeName       string
	nodeStore      listers.NodeLister
	nodeController cache.Controller
	// subnetConf                *subnet.Config
	// events                    chan subnet.Event
	// clusterCIDRController     cache.Controller
	// setNodeNetworkUnavailable bool
	// disableNodeInformer       bool
	// snFileInfo                *subnetFileInfo
}

func initk8sClient() error {
	apiUrl := "foo"
	kubeconfigPath := "/root/.kube/config"
	cfg, err := clientcmd.BuildConfigFromFlags(apiUrl, kubeconfigPath)
	if err != nil {
		return err
	}

	c, err := clientset.NewForConfig(cfg)
	if err != nil {
		return err
	}

	fmt.Println("k8s Client ok!")
	// The kube subnet mgr needs to know the k8s node name that it's running on so it can annotate it.
	// If we're running as a pod then the POD_NAME and POD_NAMESPACE will be populated and can be used to find the node
	// name. Otherwise, the environment variable NODE_NAME can be passed in.

	// 通过使用clientset初始化的c 利用k8s进行节点之间net的管理
	return nil
	// sm, err := newKubeSubnetManager(ctx, c, sc, nodeName, prefix, useMultiClusterCidr)
}

func newKubeSubnetManager(ctx context.Context, c clientset.Interface) (*kubeSubnetManager, error) {
	var ksm kubeSubnetManager
	ksm.client = c

	indexer, controller := cache.NewIndexerInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return ksm.client.CoreV1().Nodes().List(ctx, options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return ksm.client.CoreV1().Nodes().Watch(ctx, options)
			},
		},
		&v1.Node{},   // objType
		resyncPeriod, // resync time period
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {

			},
			UpdateFunc: func(oldObj, newObj interface{}) {

			},
			DeleteFunc: func(obj interface{}) {

			},
		},
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)

	ksm.nodeController = controller
	ksm.nodeStore = listers.NewNodeLister(indexer)
}
fooooooooooooooo