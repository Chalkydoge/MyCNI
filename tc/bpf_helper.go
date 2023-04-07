package tc

import (
	"fmt"
	"mycni/consts"
	"os/exec"
	"strings"
)

type BPF_TC_DIRECT string

// both directions of tc
const (
	INGRESS BPF_TC_DIRECT = "ingress"
	EGRESS  BPF_TC_DIRECT = "egress"
)

func GetVethIngressPath() string {
	return consts.K8S_CNI_PATH + "/veth_ingress.bpf.o"
}

func GetVxlanIngressPath() string {
	return consts.K8S_CNI_PATH + "/vxlan_ingress.o"
}

func GetVxlanEgressPath() string {
	return consts.K8S_CNI_PATH + "/vxlan_egress.o"
}

// Check if there is alreay bpf prog binded to device's ingress queue
func ExistOnIngress(device string) bool {
	p := exec.Command(
		"/bin/sh", "-c",
		fmt.Sprintf("tc qdisc show dev %s ingress", device),
	)
	o, _ := p.Output()
	return strings.Contains(string(o), "direct-action")
}

// Check if there is alreay bpf prog binded to device's egress queue
func ExistOnEgress(device string) bool {
	p := exec.Command(
		"/bin/sh", "-c",
		fmt.Sprintf("tc qdisc show dev %s engress", device),
	)
	o, _ := p.Output()
	return strings.Contains(string(o), "direct-action")
}

// Check whether exists qdisc on current netdev
func ExistClsact(dev string) bool {
	processInfo := exec.Command(
		"/bin/sh", "-c",
		fmt.Sprintf("tc qdisc show dev %s", dev),
	)
	out, _ := processInfo.Output()
	return strings.Contains(string(out), "clsact")
}

// Add qdisc (class&act) to netdev's queue
func AddClsact(device string) error {
	processInfo := exec.Command(
		"/bin/sh", "-c",
		fmt.Sprintf("tc qdisc add dev %s clsact", device),
	)
	_, err := processInfo.Output()
	return err
}

// Attach program to tc device,
//
// supports both ingress and egress
func AttachBPF2Device(device, prog string, dir BPF_TC_DIRECT) error {
	// If no clsact has been set up, first add qdisc
	if !ExistClsact(device) {
		err := AddClsact(device)
		if err != nil {
			return err
		}
	}

	var cmd string
	switch dir {
	case INGRESS:
		cmd = fmt.Sprintf("tc filter replace dev %s ingress handle 0x1 bpf da obj %s", device, prog)
	case EGRESS:
		cmd = fmt.Sprintf("tc filter replace dev %s egress handle 0x1 bpf da obj %s", device, prog)
	}

	p := exec.Command("/bin/sh", "-c", cmd)
	_, err := p.Output()
	return err
}

// tc entry
func AttachBPF2TC(device, prog string, direct BPF_TC_DIRECT) error {
	if !ExistClsact(device) {
		err := AddClsact(device)
		if err != nil {
			return err
		}
	}

	switch direct {
	case INGRESS:
		if ExistOnIngress(device) {
			return nil
		}
		return AttachBPF2Device(device, prog, INGRESS)
	case EGRESS:
		if ExistOnEgress(device) {
			return nil
		}
		return AttachBPF2Device(device, prog, EGRESS)
	}
	return fmt.Errorf("Unknown error: cannot attach bpf to netdevice!")
}

// Detach bpf program from certain device
func DetachBPF(device string) error {
	// tc [options] class [add|delete] dev DEV parent qdisc-id
	cmd := fmt.Sprintf("tc filter delete dev %s clsact", device)
	p := exec.Command("/bin/sh", "-c", cmd)
	_, err := p.Output()
	return err
}

// Show bpf program details attached to certain net device
//
// Direction is given by direct.
func ShowBPF(dev string, direct string) (string, error) {
	processInfo := exec.Command(
		"/bin/sh", "-c",
		fmt.Sprintf("tc filter show dev %s %s", dev, direct),
	)
	out, err := processInfo.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
