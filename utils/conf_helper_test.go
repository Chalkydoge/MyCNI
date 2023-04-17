package utils

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestGetIps(t *testing.T) {
	te := assert.New(t)

	ips, err := GetValidIps("10.1.1.0/28")
	te.NotNil(ips)
	te.Nil(err)
	te.Equal(ips[0], "10.1.1.2/28")
	
	// there should be 14 available ip address
	// exclude 10.1.1.0(neglected) & 10.1.1.1/28(default gw) 
	te.Equal(len(ips), 14)
	te.Equal(len(ips[1]), 11)
}

func TestGatewayIP(t *testing.T) {
	te := assert.New(t)

	ip := GetGateway("10.1.1.0/28")
	te.NotNil(ip)
	te.Equal(ip, "10.1.1.1/28")
}

func TestCommonGetPaths(t *testing.T) {
	te := assert.New(t)

	k8sconfpath := GetConfigPath()
	te.Equal(k8sconfpath, "/home/ubuntu/workspace/src/mycni/foo.yaml")

	masterNodeIP, err := GetMasterNodeIP()
	te.Nil(err)
	te.Equal(masterNodeIP, "https://10.28.147.22:6443")

	ipPoolPath := GetIPPoolPath()
	te.Equal(ipPoolPath, "mycni/ipam/pool")

	hostpath := GetHostPath()
	te.Equal(hostpath, "mycni/ipam/master")
	
	hostIpPoolPath := GetHostIPPoolPath()
	te.Equal(hostIpPoolPath, "mycni/ipam/master/pool")

	hostGWPath := GetHostGWPath()
	te.Equal(hostGWPath, "mycni/ipam/master/gateway")
}