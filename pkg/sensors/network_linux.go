//go:build linux

package sensors

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

func init() {
	Register(&NetworkProvider{})
}

// NetworkProvider provides network sensor data on Linux.
type NetworkProvider struct {
	interfacePattern string
}

// Meta returns the sensor metadata.
func (p *NetworkProvider) Meta() SensorMeta {
	return SensorMeta{
		ID:          "network",
		Name:        "Network",
		Description: "Network interface statistics",
		Category:    "network",
		Platforms:   []string{"linux"},
		IsArray:     true,
		ArrayKey:    "interface",
		Fields: []FieldDef{
			{Name: "Interface", JSONName: "interface", TSName: "interface", Type: FieldTypeString, Unit: "", Description: "Interface name"},
			{Name: "RxRate", JSONName: "rx_rate", TSName: "rxRate", Type: FieldTypeNumber, Unit: "B/s", Description: "Receive rate"},
			{Name: "TxRate", JSONName: "tx_rate", TSName: "txRate", Type: FieldTypeNumber, Unit: "B/s", Description: "Transmit rate"},
			{Name: "RxTotal", JSONName: "rx_total", TSName: "rxTotal", Type: FieldTypeNumber, Unit: "bytes", Description: "Total bytes received"},
			{Name: "TxTotal", JSONName: "tx_total", TSName: "txTotal", Type: FieldTypeNumber, Unit: "bytes", Description: "Total bytes transmitted"},
		},
	}
}

// Available returns true if network data can be collected.
func (p *NetworkProvider) Available() bool {
	_, err := os.Stat("/proc/net/dev")
	return err == nil
}

// Collect gathers network sensor data.
func (p *NetworkProvider) Collect(state *CollectorState) map[string]interface{} {
	now := time.Now()

	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil
	}
	defer file.Close()

	networks := make([]map[string]interface{}, 0)
	scanner := bufio.NewScanner(file)

	// Skip header lines
	scanner.Scan()
	scanner.Scan()

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}

		iface := strings.TrimSpace(parts[0])

		// Skip loopback and virtual interfaces
		if iface == "lo" || strings.HasPrefix(iface, "veth") ||
			strings.HasPrefix(iface, "docker") || strings.HasPrefix(iface, "br-") {
			continue
		}

		// Apply interface filter
		if p.interfacePattern != "" && p.interfacePattern != "*" {
			if !strings.Contains(iface, p.interfacePattern) {
				continue
			}
		}

		fields := strings.Fields(parts[1])
		if len(fields) < 10 {
			continue
		}

		rxBytes, _ := strconv.ParseUint(fields[0], 10, 64)
		txBytes, _ := strconv.ParseUint(fields[8], 10, 64)

		// Get previous values for rate calculation
		stateKey := "network_" + iface
		type netSample struct {
			rxBytes uint64
			txBytes uint64
			time    time.Time
		}

		var rxRate, txRate float64
		if prev, ok := GetTyped[netSample](state, stateKey); ok {
			elapsed := now.Sub(prev.time).Seconds()
			if elapsed > 0 && elapsed < 5 {
				rxRate = float64(rxBytes-prev.rxBytes) / elapsed
				txRate = float64(txBytes-prev.txBytes) / elapsed
			}
		}

		state.Set(stateKey, netSample{rxBytes: rxBytes, txBytes: txBytes, time: now})

		networks = append(networks, map[string]interface{}{
			"interface": iface,
			"rx_rate":   rxRate,
			"tx_rate":   txRate,
			"rx_total":  rxBytes,
			"tx_total":  txBytes,
		})
	}

	return map[string]interface{}{
		"_items": networks,
	}
}

// SetInterfacePattern sets the interface filter pattern.
func (p *NetworkProvider) SetInterfacePattern(pattern string) {
	p.interfacePattern = pattern
}

// NewNetworkProvider creates a network provider with specific interface filter.
func NewNetworkProvider(pattern string) *NetworkProvider {
	return &NetworkProvider{interfacePattern: pattern}
}
