package sensors

import (
	"os"
)

func init() {
	Register(&HostnameProvider{})
}

// HostnameProvider provides hostname/device name.
type HostnameProvider struct{}

// Meta returns the sensor metadata.
func (p *HostnameProvider) Meta() SensorMeta {
	return SensorMeta{
		ID:          "hostname",
		Name:        "Hostname",
		Description: "System hostname and device name",
		Category:    "system",
		Platforms:   []string{"linux", "darwin", "windows"},
		Fields: []FieldDef{
			{Name: "Hostname", JSONName: "hostname", TSName: "hostname", Type: FieldTypeString, Unit: "", Description: "System hostname"},
		},
	}
}

// Available returns true - hostname is always available.
func (p *HostnameProvider) Available() bool {
	return true
}

// Collect gathers hostname.
func (p *HostnameProvider) Collect(state *CollectorState) map[string]interface{} {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "Unknown"
	}

	return map[string]interface{}{
		"hostname": hostname,
	}
}
