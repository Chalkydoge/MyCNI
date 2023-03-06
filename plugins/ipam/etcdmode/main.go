package main

import (
	"fmt"
	"mycni/utils"

	// "github.com/containernetworking/cni/pkg/skel"
	// "github.com/containernetworking/cni/pkg/types"
	// current "github.com/containernetworking/cni/pkg/types/100"
	// "github.com/containernetworking/cni/pkg/version"
)

// "192.168.64.0/24", &IPAMOptions{
// 	RangeStart: "192.168.64.10",
// 	RangeEnd:   "192.168.64.20",
// }

// subnet = IP4CIDR like 10.1.1.0/28
// IPAM Options, like support rangeStart - rangeEnd
func main() {
	fmt.Println(utils.GetHostPath())
	fmt.Println(utils.GetIPPoolPath())
}

// func main() {
	// skel.PluginMain(cmdAdd, cmdCheck, cmdDel, version.All, bv.BuildString("host-local"))
// }

// func cmdAdd(args *skel.CmdArgs) error {
// 	ipamConf, confVersion, err := allocator.LoadIPAMConfig(args.StdinData, args.Args)
// 	if err != nil {
// 		return err
// 	}
// 	result := &current.Result{CNIVersion: current.ImplementedSpecVersion}

// 	// no dns here

// 	// new store here

// 	// since we use etcd for ip allocation, we don't need to store it locally.

// 	ipConf, err := allocator.Get(args.ContainerID, args.IfName)
// 	if err != nil {
// 		// Deallocate all already allocated IPs
// 		for _, alloc := range allocs {
// 			_ = alloc.Release(args.ContainerID, args.IfName)
// 		}
// 		return fmt.Errorf("failed to allocate for range %d: %v", idx, err)
// 	}

// 	allocs = append(allocs, allocator)
// 	result.IPs = append(result.IPs, ipConf)
	
// 	result.Routes = ipamConf.Routes

// 	return types.PrintResult(result, confVersion)
// }
 