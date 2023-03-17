package tc

import (
	"fmt"
	"os/exec"
	"strings"
)

type BPF_TC_DIRECT string

// both directions of tc
const (
	INGRESS BPF_TC_DIRECT = "ingress"
	EGRESS  BPF_TC_DIRECT = "egress"
)

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

// Attach program to tc device,
//
// supports both ingress and egress
func AttachBPF2Device(device, prog string, dir BPF_TC_DIRECT) error {
	var cmd string
	switch dir {
	case INGRESS:
		cmd = fmt.Sprintf("tc filter add dev %s ingress bpf direct-action obj %s", device, prog)
	case EGRESS:
		cmd = fmt.Sprintf("tc filter add dev %s egress bpf direct-action obj %s", device, prog)
	}

	p := exec.Command("/bin/sh", "-c", cmd)
	_, err := p.Output()
	return err
}

// tc entry
func AttachBPF2TC(device, prog string, direct BPF_TC_DIRECT) error {
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
