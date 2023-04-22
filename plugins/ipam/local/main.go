package main

import (
	"errors"
	"fmt"
	"net"
	"runtime"

	"mycni/pkg/config"
	"mycni/plugins/ipam/local/store"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	cip "github.com/containernetworking/plugins/pkg/ip"
	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
)

type IPAM struct {
	subnet  *net.IPNet
	gateway net.IP
	store   *store.Store
}

var (
	IPOverflowError = errors.New("ip overflow")
)

const (
	ClusterCIDR = "10.244.0.0/16"
)

func init() {
	runtime.LockOSThread()
}

func main() {
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, bv.BuildString("local"))
}

// 根据cni配置初始化ipam
func NewIPAM(conf *config.CNIConf, s *store.Store) (*IPAM, error) {
	_, ipnet, err := net.ParseCIDR(conf.Subnet)
	if err != nil {
		return nil, err
	}

	im := &IPAM{
		subnet: ipnet,
		store:  s,
	}

	// gateway = 子网段的第一个ip
	im.gateway, err = im.NextIP(im.subnet.IP)
	if err != nil {
		return nil, err
	}

	return im, nil
}

func (im *IPAM) Mask() net.IPMask {
	return im.subnet.Mask
}

func (im *IPAM) Gateway() net.IP {
	return im.gateway
}

func (im *IPAM) IPNet(ip net.IP) *net.IPNet {
	return &net.IPNet{IP: ip, Mask: im.Mask()}
}

func (im *IPAM) NextIP(ip net.IP) (net.IP, error) {
	next := cip.NextIP(ip)
	if !im.subnet.Contains(next) {
		return nil, IPOverflowError
	}

	return next, nil
}

func (im *IPAM) AllocateIP(id, ifName string) (net.IP, error) {
	im.store.Lock()
	defer im.store.Unlock()

	if err := im.store.LoadData(); err != nil {
		return nil, err
	}

	// 已经分配了ip 跳过
	ip, _ := im.store.GetIPByID(id)
	if len(ip) > 0 {
		return ip, nil
	}

	// 上一个已经分配的地址
	last := im.store.Last()
	if len(last) == 0 {
		last = im.gateway
	}

	start := make(net.IP, len(last))
	copy(start, last)
	for {
		next, err := im.NextIP(start)
		if err == IPOverflowError && !last.Equal(im.gateway) {
			start = im.gateway
			continue
		} else if err != nil {
			return nil, err
		}

		if !im.store.Contain(next) {
			err := im.store.Add(next, id, ifName)
			return next, err
		}

		start = next
		if start.Equal(last) {
			break
		}
	}

	return nil, fmt.Errorf("no available ip")
}

func (im *IPAM) ReleaseIP(id string) error {
	im.store.Lock()
	defer im.store.Unlock()

	if err := im.store.LoadData(); err != nil {
		return err
	}

	return im.store.Del(id)
}

func (im *IPAM) CheckIP(id string) (net.IP, error) {
	im.store.RLock()
	defer im.store.RUnlock()

	if err := im.store.LoadData(); err != nil {
		return nil, err
	}

	ip, ok := im.store.GetIPByID(id)
	if !ok {
		return nil, fmt.Errorf("failed to find container %s ip", id)
	}

	return ip, nil
}

func cmdAdd(args *skel.CmdArgs) error {
	cniConf, err := config.LoadCNIConfig(args.StdinData)
	if err != nil {
		return err
	}

	s, err := store.NewStore(cniConf.DataDir, cniConf.Name)
	if err != nil {
		return err
	}
	defer s.Close()

	ipam, err := NewIPAM(cniConf, s)
	if err != nil {
		return err
	}

	gateway := ipam.Gateway()
	allocated_ip, err := ipam.AllocateIP(args.ContainerID, args.IfName)
	if err != nil {
		return err
	}

	_, cidr, err := net.ParseCIDR(ClusterCIDR)
	if err != nil {
		return err
	}

	// 这里额外添加一条路由规则 用于跨节点的通信情况
	result := &current.Result{
		CNIVersion: current.ImplementedSpecVersion,
		IPs: []*current.IPConfig{
			{
				Address: net.IPNet{IP: allocated_ip, Mask: ipam.Mask()},
				Gateway: gateway,
			},
		},
		Routes: []*types.Route{
			{
				Dst: net.IPNet{IP: cidr.IP, Mask: cidr.Mask},
				GW:  gateway,
			},
		},
	}
	return types.PrintResult(result, cniConf.CNIVersion)
}

func cmdCheck(args *skel.CmdArgs) error {
	conf, err := config.LoadCNIConfig(args.StdinData)
	if err != nil {
		return err
	}

	s, err := store.NewStore(conf.DataDir, conf.Name)
	if err != nil {
		return err
	}
	defer s.Close()

	ipam, err := NewIPAM(conf, s)
	if err != nil {
		return fmt.Errorf("failed to create ipam: %v", err)
	}

	_, err = ipam.CheckIP(args.ContainerID)
	if err != nil {
		return err
	}

	return nil
}

func cmdDel(args *skel.CmdArgs) error {
	cniConf, err := config.LoadCNIConfig(args.StdinData)
	if err != nil {
		return err
	}

	s, err := store.NewStore(cniConf.DataDir, cniConf.Name)
	if err != nil {
		return err
	}
	defer s.Close()

	ipam, err := NewIPAM(cniConf, s)
	if err != nil {
		return fmt.Errorf("failed to create ipam: %v", err)
	}
	if err := ipam.ReleaseIP(args.ContainerID); err != nil {
		return err
	}
	return nil
}
