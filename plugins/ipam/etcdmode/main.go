package main

import (
	"fmt"
	"strings"
	"mycni/etcdwrap"
	"mycni/plugins/ipam/etcdmode/initpool"
	"mycni/plugins/ipam/etcdmode/allocator"

	bv "github.com/containernetworking/plugins/pkg/utils/buildversion"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
)

func main() {
	skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, bv.BuildString("etcdmode"))
}

func cmdAdd(args *skel.CmdArgs) error {
	// first load cni conf, with ipam config
	// args.StdinData: json conf
	// args.Args: string
	_, confVersion, err := allocator.LoadIPAMConfig(args.StdinData, args.Args)
	if err != nil {
		return err
	}
	result := &current.Result{CNIVersion: current.ImplementedSpecVersion}

	// no dns here
	// new store here
	// since we use etcd for ip allocation, we don't need to store it locally.
	etcdwrap.Init()
	cli, err := etcdwrap.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("failed to boot etcd client!")
	}
	
	// init pool first
	poolready := cli.GetInitPoolStatus()
	if poolready == false {
		_, err := initpool.InitPool(cli)
		if err != nil {
			return fmt.Errorf("Failed to initip pool")
		}
	}

	ipConf, err := allocator.AllocateIP2Pod(args.ContainerID, args.IfName, cli)
	if err != nil {
		// TODO: Deallocate all already allocated IPs
		_, _ = allocator.ReleasePodIP(args.ContainerID, args.IfName, cli)
		return fmt.Errorf("failed to allocate for container %s, err is %v", args.ContainerID, err)
	}

	result.IPs = append(result.IPs, ipConf)
	return types.PrintResult(result, confVersion)
}

func cmdCheck(args *skel.CmdArgs) error {
	_, _, err := allocator.LoadIPAMConfig(args.StdinData, args.Args)
	if err != nil {
		return err
	}

	// See if the container has been properly allocated with ip
	cli, err := etcdwrap.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("Failed to bootup etcd client! Error is %v", err)
	}
	containerIpFound, err := allocator.FindByID(args.ContainerID, args.IfName, cli)
	if err != nil || containerIpFound == false {
		return fmt.Errorf("etcdmode: Failed to find address added by container %v", args.ContainerID)
	}
	return nil
}

func cmdDel(args *skel.CmdArgs) error {
	cli, err := etcdwrap.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("Failed to bootup etcd client! Error is %v", err)
	}
	// Loop through all ranges, releasing all IPs, even if an error occurs
	var errors []string	
	_, err = allocator.ReleasePodIP(args.ContainerID, args.IfName, cli)
	if err != nil {
		errors = append(errors, err.Error())
	}

	if errors != nil {
		return fmt.Errorf(strings.Join(errors, ";"))
	}
	
	return nil
}
