package allocator

// The top-level network config - IPAM plugins are passed the full configuration
// of the calling plugin, not just the IPAM section.
type Net struct {
	Name          string      `json:"name"`
	CNIVersion    string      `json:"cniVersion"`
	IPAM          *IPAMConfig `json:"ipam"`
	RuntimeConfig struct {
		// The capability arg
		IPRanges []RangeSet `json:"ipRanges,omitempty"`
		IPs      []*ip.IP   `json:"ips,omitempty"`
	} `json:"runtimeConfig,omitempty"`
	// Args *struct {
	// 	A *IPAMArgs `json:"cni"`
	// } `json:"args"`
}

// IPAMConfig represents the IP related network configuration.
// This nests Range because we initially only supported a single
// range directly, and wish to preserve backwards compatability
type IPAMConfig struct {
	Name       string
	Type       string         `json:"type"`
	Routes     []*types.Route `json:"routes,omitempty"`
	DataDir    string         `json:"dataDir,omitempty""`
	ResolvConf string         `json:"resolvConf,omitempty""`
	Ranges     []RangeSet     `json:"ranges,omitempty""`
}

// NewIPAMConfig creates a NetworkConfig from the given network name.
func LoadIPAMConfig(bytes []byte) (*IPAMConfig, string, error) {
	n := Net{}
	if err := json.Unmarshal(bytes, &n); err != nil {
		return nil, "", err
	}

	if n.IPAM == nil {
		return nil, "", fmt.Errorf("IPAM config missing 'ipam' key")
	}

	// Copy net name into IPAM so not to drag Net struct around
	n.IPAM.Name = n.Name

	return n.IPAM, n.CNIVersion, nil
}

