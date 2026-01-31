//go:build darwin

package sensors

// Darwin (macOS) stub providers
// These sensors are not yet implemented for macOS.
// They register but return Available() = false.

func init() {
	Register(&darwinCPUProvider{})
	Register(&darwinMemoryProvider{})
	Register(&darwinDiskProvider{})
	Register(&darwinNetworkProvider{})
}

// darwinCPUProvider is a stub for macOS CPU monitoring.
type darwinCPUProvider struct{}

func (p *darwinCPUProvider) Meta() SensorMeta {
	return SensorMeta{
		ID:          "cpu",
		Name:        "CPU",
		Description: "CPU usage, temperature, and frequency (macOS)",
		Category:    "system",
		Platforms:   []string{"darwin"},
		Fields: []FieldDef{
			{Name: "Load", JSONName: "load", TSName: "load", Type: FieldTypeNumber, Unit: "%", Description: "CPU load percentage"},
			{Name: "Temperature", JSONName: "temperature", TSName: "temperature", Type: FieldTypeOptionalNumber, Unit: "°C", Description: "CPU temperature"},
			{Name: "Frequency", JSONName: "frequency", TSName: "frequency", Type: FieldTypeOptionalNumber, Unit: "MHz", Description: "CPU frequency"},
			{Name: "Cores", JSONName: "cores", TSName: "cores", Type: FieldTypeNumber, Unit: "", Description: "Number of CPU cores"},
		},
	}
}

func (p *darwinCPUProvider) Available() bool {
	// TODO: Implement macOS CPU monitoring using sysctl
	return false
}

func (p *darwinCPUProvider) Collect(state *CollectorState) map[string]interface{} {
	return nil
}

// darwinMemoryProvider is a stub for macOS memory monitoring.
type darwinMemoryProvider struct{}

func (p *darwinMemoryProvider) Meta() SensorMeta {
	return SensorMeta{
		ID:          "memory",
		Name:        "Memory",
		Description: "System memory (RAM) usage (macOS)",
		Category:    "system",
		Platforms:   []string{"darwin"},
		Fields: []FieldDef{
			{Name: "Total", JSONName: "total", TSName: "total", Type: FieldTypeNumber, Unit: "MB", Description: "Total memory"},
			{Name: "Used", JSONName: "used", TSName: "used", Type: FieldTypeNumber, Unit: "MB", Description: "Used memory"},
			{Name: "Available", JSONName: "available", TSName: "available", Type: FieldTypeNumber, Unit: "MB", Description: "Available memory"},
			{Name: "Percent", JSONName: "percent", TSName: "percent", Type: FieldTypeNumber, Unit: "%", Description: "Memory usage percentage"},
		},
	}
}

func (p *darwinMemoryProvider) Available() bool {
	// TODO: Implement macOS memory monitoring using sysctl/host_statistics
	return false
}

func (p *darwinMemoryProvider) Collect(state *CollectorState) map[string]interface{} {
	return nil
}

// darwinDiskProvider provides disk stats on macOS using statfs.
type darwinDiskProvider struct {
	mounts []string
}

// defaultDiskMounts returns the default disk mount points for macOS.
func defaultDiskMounts() []string {
	return []string{"/"}
}

func (p *darwinDiskProvider) Meta() SensorMeta {
	return SensorMeta{
		ID:          "disk",
		Name:        "Disk",
		Description: "Disk usage statistics (macOS)",
		Category:    "storage",
		Platforms:   []string{"darwin"},
		IsArray:     true,
		ArrayKey:    "mount",
		Fields: []FieldDef{
			{Name: "Mount", JSONName: "mount", TSName: "mount", Type: FieldTypeString, Unit: "", Description: "Mount point"},
			{Name: "Total", JSONName: "total", TSName: "total", Type: FieldTypeNumber, Unit: "GB", Description: "Total disk space"},
			{Name: "Used", JSONName: "used", TSName: "used", Type: FieldTypeNumber, Unit: "GB", Description: "Used disk space"},
			{Name: "Free", JSONName: "free", TSName: "free", Type: FieldTypeNumber, Unit: "GB", Description: "Free disk space"},
			{Name: "Percent", JSONName: "percent", TSName: "percent", Type: FieldTypeNumber, Unit: "%", Description: "Disk usage percentage"},
		},
	}
}

func (p *darwinDiskProvider) Available() bool {
	return true // statfs works on macOS
}

// Configure applies the given config to the provider.
func (p *darwinDiskProvider) Configure(config *Config) {
	if mounts, ok := config.GetStringSliceOption("disk.mounts"); ok {
		p.mounts = mounts
	}
}

// Options returns the configuration options for this provider.
func (p *darwinDiskProvider) Options() []OptionDef {
	return []OptionDef{
		{
			Key:         "disk.mounts",
			Type:        "[]string",
			Default:     "/ (Linux/macOS), C:\\ (Windows)",
			Description: "Disk mount points to monitor",
			Example:     "--opt disk.mounts=/,/home,/data",
		},
	}
}

func (p *darwinDiskProvider) Collect(state *CollectorState) map[string]interface{} {
	mounts := p.mounts
	if len(mounts) == 0 {
		mounts = defaultDiskMounts()
	}

	disks := make([]map[string]interface{}, 0, len(mounts))

	for _, mount := range mounts {
		var stat syscallStatfs
		if err := statfs(mount, &stat); err != nil {
			continue
		}

		blockSize := uint64(stat.Bsize)
		totalBytes := stat.Blocks * blockSize
		freeBytes := stat.Bfree * blockSize
		availBytes := stat.Bavail * blockSize
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

// darwinNetworkProvider is a stub for macOS network monitoring.
type darwinNetworkProvider struct{}

func (p *darwinNetworkProvider) Meta() SensorMeta {
	return SensorMeta{
		ID:          "network",
		Name:        "Network",
		Description: "Network interface statistics (macOS)",
		Category:    "network",
		Platforms:   []string{"darwin"},
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

func (p *darwinNetworkProvider) Available() bool {
	// TODO: Implement macOS network monitoring using netstat or sysctl
	return false
}

func (p *darwinNetworkProvider) Collect(state *CollectorState) map[string]interface{} {
	return nil
}
