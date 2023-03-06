package utils

import(
	"os"
	"net"
	"fmt"
	"strings"
	"io/ioutil"
	"encoding/binary"

	"mycni/consts"
	"github.com/dlclark/regexp2"
)

func invalidIP(ip string) bool {
	parts := strings.Split(ip, ".")
	n := len(parts) - 1	
	if parts[n] == "0" || parts[n] == "1" || parts[n] == "255" {
		return true
	}
	return false
}

// given cidr, list all ip address under this subnet
// return: an ip list
func listIPv4Addr(cidr string) ([]string, error) {
	_, ipv4Net, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("listIPv4Addr failed! %v", err)
	}

	mask := binary.BigEndian.Uint32(ipv4Net.Mask)
	start := binary.BigEndian.Uint32(ipv4Net.IP)
	fin := (start & mask) | (mask ^ 0xffffffff)

	var ips []string
	for i := start; i <= fin; i++ {
		ip := make(net.IP, 4)
		binary.BigEndian.PutUint32(ip, i)
		
		// ip address => string here
		ip_s := ip.String()
		if invalidIP(ip_s) { 
			continue
		}
		ips = append(ips, ip_s)
	}
	return ips, nil
}

// Check whether path exist on the Node
func PathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

// Get config path
// admin.conf; kubelet.conf; ~/.kube/conf
func GetConfigPath() string {
	confPath := consts.K8S_LOCAL_DEFAULT_PATH
	
	if PathExists(consts.K8S_ADMIN_CONF_PATH) {
		confPath = consts.K8S_ADMIN_CONF_PATH
	} else if PathExists(consts.K8S_KUBELET_CONF_PATH) {
		confPath = consts.K8S_KUBELET_CONF_PATH
	}
	return confPath
}

// get master node ip from conf
func GetMasterNodeIP() (string, error) {
	confPath := GetConfigPath()
	confByte, err := ioutil.ReadFile(confPath)

	if err != nil {
		Log("Reading conf from path ", confPath, " Failed! Err is: ", err.Error())
		return "", err
	}

	// server: xx.xx.xx.xx from k8s conf
	masterIP, err := GetLineFromYaml(string(confByte), "server")
	if err != nil {
		Log("Get master node ip from ", confPath, " Failed! Err is: ", err.Error())
		return "", err
	}

	return masterIP, nil
}

// get aa: [bb] line info from yaml
func GetLineFromYaml(yml string, key string) (string, error) {
	r, err := regexp2.Compile(fmt.Sprintf(`(?<=%s: )(.*)`, key), 0)
	if err != nil {
		Log("Init regexp2 failed! ", err.Error())
		return "", err
	}

	res, err := r.FindStringMatch(yml)
	if err != nil {
		Log("Match ip failed! ", err.Error())
		return "", err
	}

	return res.String(), nil
}

// get host path in etcd, mycni/ipam/hostname
func GetHostPath() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "/test-error-path"
	}
	return consts.ETCD_COMMON_PREFIX + hostname
}

// get alls ip pool path in etcd
func GetIPPoolPath() string {
	return consts.ETCD_COMMON_PREFIX + "pool"
}

// get current host's ip pool path in etcd
func GetHostIPPoolPath() string {
	return GetHostPath() + "/pool"
}

// get current host's gateway ip
func GetHostGWPath() string {
	return GetHostPath() + "/gateway"
}

// get gateway according to given ip
func GetGateway(givenIP string) string {
	segs := strings.Split(givenIP, "/")
	ip_part := strings.Split(segs[0], ".")

	n := len(ip_part)
	ip_part[n - 1] = "1"
	return strings.Join(ip_part, ".")
}

// given a ip cidr, return useable ip addresses, (ignore 0, 1, 255)
func GetValidIps(ipcidr string) ([]string, error) {
	return listIPv4Addr(ipcidr)
}

// split raw value into array
func ConvertString2Array(s string) []string {
	return strings.Split(s, ";")
}

// combine array value into string
func ConvertArray2String(arr []string) string {
	return strings.Join(arr, ";")
}
