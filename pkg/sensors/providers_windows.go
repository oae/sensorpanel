//go:build windows

package sensors

// Windows stub providers
// These sensors are not yet fully implemented for Windows.
// They register but most return Available() = false.

func init() {
	Register(&windowsCPUProvider{})
	Register(&windowsMemoryProvider{})
	Register(&windowsDiskProvider{})
	Register(&windowsNetworkProvider{})
}

// windowsCPUProvider is a stub for Windows CPU monitoring.
type windowsCPUProvider struct{}

func (p *windowsCPUProvider) Meta() SensorMeta {
	return SensorMeta{
		ID:          "cpu",
		Name:        "CPU",
		Description: "CPU usage, temperature, and frequency (Windows)",
		Category:    "system",
		Platforms:   []string{"windows"},
		Fields: []FieldDef{
			{Name: "Load", JSONName: "load", TSName: "load", Type: FieldTypeNumber, Unit: "%", Description: "CPU load percentage"},
			{Name: "Temperature", JSONName: "temperature", TSName: "temperature", Type: FieldTypeOptionalNumber, Unit: "°C", Description: "CPU temperature"},
			{Name: "Frequency", JSONName: "frequency", TSName: "frequency", Type: FieldTypeOptionalNumber, Unit: "MHz", Description: "CPU frequency"},
			{Name: "Cores", JSONName: "cores", TSName: "cores", Type: FieldTypeNumber, Unit: "", Description: "Number of CPU cores"},
		},
	}
}

func (p *windowsCPUProvider) Available() bool {
	// TODO: Implement Windows CPU monitoring using WMI or PDH
	return false
}

func (p *windowsCPUProvider) Collect(state *CollectorState) map[string]interface{} {
	return nil
}

// windowsMemoryProvider is a stub for Windows memory monitoring.
type windowsMemoryProvider struct{}

func (p *windowsMemoryProvider) Meta() SensorMeta {
	return SensorMeta{
		ID:          "memory",
		Name:        "Memory",
		Description: "System memory (RAM) usage (Windows)",
		Category:    "system",
		Platforms:   []string{"windows"},
		Fields: []FieldDef{
			{Name: "Total", JSONName: "total", TSName: "total", Type: FieldTypeNumber, Unit: "MB", Description: "Total memory"},
			{Name: "Used", JSONName: "used", TSName: "used", Type: FieldTypeNumber, Unit: "MB", Description: "Used memory"},
			{Name: "Available", JSONName: "available", TSName: "available", Type: FieldTypeNumber, Unit: "MB", Description: "Available memory"},
			{Name: "Percent", JSONName: "percent", TSName: "percent", Type: FieldTypeNumber, Unit: "%", Description: "Memory usage percentage"},
		},
	}
}

func (p *windowsMemoryProvider) Available() bool {
	// TODO: Implement Windows memory monitoring using GlobalMemoryStatusEx
	return false
}

func (p *windowsMemoryProvider) Collect(state *CollectorState) map[string]interface{} {
	return nil
}

// windowsDiskProvider provides disk stats on Windows using GetDiskFreeSpaceEx.
type windowsDiskProvider struct {
	mounts []string
}

func (p *windowsDiskProvider) Meta() SensorMeta {
	return SensorMeta{
		ID:          "disk",
		Name:        "Disk",
		Description: "Disk usage statistics (Windows)",
		Category:    "storage",
		Platforms:   []string{"windows"},
		IsArray:     true,
		ArrayKey:    "mount",
		Fields: []FieldDef{
			{Name: "Mount", JSONName: "mount", TSName: "mount", Type: FieldTypeString, Unit: "", Description: "Drive letter"},
			{Name: "Total", JSONName: "total", TSName: "total", Type: FieldTypeNumber, Unit: "GB", Description: "Total disk space"},
			{Name: "Used", JSONName: "used", TSName: "used", Type: FieldTypeNumber, Unit: "GB", Description: "Used disk space"},
			{Name: "Free", JSONName: "free", TSName: "free", Type: FieldTypeNumber, Unit: "GB", Description: "Free disk space"},
			{Name: "Percent", JSONName: "percent", TSName: "percent", Type: FieldTypeNumber, Unit: "%", Description: "Disk usage percentage"},
		},
	}
}

func (p *windowsDiskProvider) Available() bool {
	return true // GetDiskFreeSpaceEx works on Windows
}

func (p *windowsDiskProvider) Collect(state *CollectorState) map[string]interface{} {
	mounts := p.mounts
	if len(mounts) == 0 {
		mounts = []string{"C:\\"}
	}

	disks := make([]map[string]interface{}, 0, len(mounts))

	for _, mount := range mounts {
		var stat syscallStatfs
		if err := statfs(mount, &stat); err != nil {
			continue
		}

		// Windows statfs returns bytes directly (Bsize=1)
		totalBytes := stat.Blocks
		freeBytes := stat.Bfree
		availBytes := stat.Bavail
		usedBytes := totalBytes - freeBytes

		totalGB := float64(totalBytes) / (1024 * 1024 * 1024)
		usedGB := float64(usedBytes) / (1024 * 1024 * 1024)
		freeGB := float64(availBytes) / (1024 * 1024 * 1024)

		var percent float64
		if totalGB > 0 {
			percent = (usedGB / totalGB) * 100.0
		}

		disks = append(disks, map[string]interface{}{
			"mount":   mount,
			"total":   totalGB,
			"used":    usedGB,
			"free":    freeGB,
			"percent": percent,
		})
	}

	return map[string]interface{}{
		"_items": disks,
	}
}

// windowsNetworkProvider is a stub for Windows network monitoring.
type windowsNetworkProvider struct{}

func (p *windowsNetworkProvider) Meta() SensorMeta {
	return SensorMeta{
		ID:          "network",
		Name:        "Network",
		Description: "Network interface statistics (Windows)",
		Category:    "network",
		Platforms:   []string{"windows"},
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

func (p *windowsNetworkProvider) Available() bool {
	// TODO: Implement Windows network monitoring using GetIfTable2
	return false
}

func (p *windowsNetworkProvider) Collect(state *CollectorState) map[string]interface{} {
	return nil
}
