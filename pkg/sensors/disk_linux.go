//go:build linux

package sensors

// defaultDiskMounts returns the default disk mount points for Linux.
func defaultDiskMounts() []string {
	return []string{"/"}
}

func init() {
	Register(&DiskProvider{})
}

// DiskProvider provides disk usage sensor data on Linux.
type DiskProvider struct {
	mounts []string
	labels map[string]string
}

// Meta returns the sensor metadata.
func (p *DiskProvider) Meta() SensorMeta {
	return SensorMeta{
		ID:          "disk",
		Name:        "Disk",
		Description: "Disk usage statistics",
		Category:    "storage",
		Platforms:   []string{"linux"},
		IsArray:     true,
		ArrayKey:    "mount",
		Fields: []FieldDef{
			{Name: "Mount", JSONName: "mount", TSName: "mount", Type: FieldTypeString, Unit: "", Description: "Mount point"},
			{Name: "Label", JSONName: "label", TSName: "label", Type: FieldTypeString, Unit: "", Description: "Display label (alias or mount point)"},
			{Name: "Total", JSONName: "total", TSName: "total", Type: FieldTypeNumber, Unit: "GB", Description: "Total disk space"},
			{Name: "Used", JSONName: "used", TSName: "used", Type: FieldTypeNumber, Unit: "GB", Description: "Used disk space"},
			{Name: "Free", JSONName: "free", TSName: "free", Type: FieldTypeNumber, Unit: "GB", Description: "Free disk space"},
			{Name: "Percent", JSONName: "percent", TSName: "percent", Type: FieldTypeNumber, Unit: "%", Description: "Disk usage percentage"},
		},
	}
}

// Available returns true if disk data can be collected.
func (p *DiskProvider) Available() bool {
	return true
}

// Configure applies the given config to the provider.
func (p *DiskProvider) Configure(config *Config) {
	if mounts, ok := config.GetStringSliceOption("disk.mounts"); ok {
		p.mounts = mounts
	}
	if labels, ok := config.GetStringMapOption("disk.labels"); ok {
		p.labels = labels
	}
}

// Options returns the configuration options for this provider.
func (p *DiskProvider) Options() []OptionDef {
	return []OptionDef{
		{
			Key:         "disk.mounts",
			Type:        "[]string",
			Default:     "/ (Linux/macOS), C:\\ (Windows)",
			Description: "Disk mount points to monitor",
			Example:     "--opt disk.mounts=/,/home,/data",
		},
		{
			Key:         "disk.labels",
			Type:        "map[string]string",
			Default:     "(none)",
			Description: "Custom labels for mount points",
			Example:     "In config.json: \"disk.labels\": {\"/\": \"Root\", \"/home\": \"Home\"}",
		},
	}
}

// Collect gathers disk sensor data.
func (p *DiskProvider) Collect(state *CollectorState) map[string]interface{} {
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

		// Use custom label if configured, otherwise use mount point
		label := mount
		if p.labels != nil {
			if l, ok := p.labels[mount]; ok {
				label = l
			}
		}

		disks = append(disks, map[string]interface{}{
			"mount":   mount,
			"label":   label,
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
