//go:build linux

package sensors

func init() {
	Register(&DiskProvider{})
}

// DiskProvider provides disk usage sensor data on Linux.
type DiskProvider struct {
	mounts []string
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

// Collect gathers disk sensor data.
func (p *DiskProvider) Collect(state *CollectorState) map[string]interface{} {
	mounts := p.mounts
	if len(mounts) == 0 {
		mounts = []string{"/"}
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

// SetMounts sets the mount points to monitor.
func (p *DiskProvider) SetMounts(mounts []string) {
	p.mounts = mounts
}

// NewDiskProvider creates a disk provider with specific mount points.
func NewDiskProvider(mounts []string) *DiskProvider {
	return &DiskProvider{mounts: mounts}
}
