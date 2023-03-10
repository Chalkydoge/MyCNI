package allocator

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestLoadConfig(t *testing.T) {
	input := `{
		"cniVersion": "0.3.1",
		"name": "mynet",
		"type": "xyz",
		"master": "foobar0",
		"ipam": {
			"type": "etcdmode"
		}
	}`
	te := assert.New(t)
	
	// parsing a cni conf
	ipamconf, cniversion, err := LoadIPAMConfig([]byte(input), "")
	te.Nil(err)
	te.Equal(cniversion, "0.3.1")
	
	// ipam conf should be same
	te.Equal(ipamconf, &IPAMConfig{
		Name: "mynet",
		Type: "etcdmode",
	})
}