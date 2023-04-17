package config
import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/containernetworking/cni/pkg/types"
)

const (
	DefaultSubnetFile = "/run/testcni/subnet.json"
	DefaultBridgeName = "testcni0" // avoid conflict with other cni plugins
)

type SubnetConf struct {
	Subnet string `json:"subnet"`
	Bridge string `json:"bridge"`
}

// PluginConf is whatever you expect your configuration json to be. This is whatever
// is passed in on stdin. Your plugin may wish to expose its functionality via
// runtime args, see CONVENTIONS.md in the CNI spec.
type PluginConf struct {
	// This embeds the standard NetConf structure which allows your plugin
	// to more easily parse standard fields like Name, Type, CNIVersion,
	// and PrevResult.
	types.NetConf

	RuntimeConfig *struct {
		Config map[string]interface{} `json:"config,omitempty"`
	} `json:"runtimeConfig,omitempty"`
	Args *struct {
		A map[string]interface{} `json:"cni,omitempty"`
	} `json:"args,omitempty"`

	// Add plugin-specifc flags here
	DataDir string `json:"dataDir"`
}

type CNIConf struct {
	PluginConf
	SubnetConf
}


func LoadSubnetConfig() (*SubnetConf, error) {
	data, err := os.ReadFile(DefaultSubnetFile)
	if err != nil {
		return nil, err
	}

	conf := &SubnetConf{}
	if err := json.Unmarshal(data, conf); err != nil {
		return nil, err
	}

	return conf, nil
}

func StoreSubnetConfig(conf *SubnetConf) error {
	data, err := json.Marshal(conf)
	if err != nil {
		return err
	}

	return os.WriteFile(DefaultSubnetFile, data, 0644)
}

func parsePluginConfig(stdin []byte) (*PluginConf, error) {
	conf := PluginConf{}

	if err := json.Unmarshal(stdin, &conf); err != nil {
		return nil, fmt.Errorf("failed to parse network configuration: %v", err)
	}

	return &conf, nil
}

func LoadCNIConfig(stdin []byte) (*CNIConf, error) {
	pluginConf, err := parsePluginConfig(stdin)
	if err != nil {
		return nil, err
	}
	subnetConf, err := LoadSubnetConfig()
	if err != nil {
		return nil, err
	}

	return &CNIConf{PluginConf: *pluginConf, SubnetConf: *subnetConf}, nil
}
