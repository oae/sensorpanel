package sensors

import (
	"runtime"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.ShowCPU {
		t.Error("ShowCPU should be true by default")
	}
	if !cfg.ShowGPU {
		t.Error("ShowGPU should be true by default")
	}
	if !cfg.ShowRAM {
		t.Error("ShowRAM should be true by default")
	}
	if !cfg.ShowDisk {
		t.Error("ShowDisk should be true by default")
	}
	if !cfg.ShowNetwork {
		t.Error("ShowNetwork should be true by default")
	}
	if len(cfg.DiskMounts) != 1 || cfg.DiskMounts[0] != "/" {
		t.Errorf("DiskMounts = %v, want [\"/\"]", cfg.DiskMounts)
	}
	if cfg.NetworkInterface != "*" {
		t.Errorf("NetworkInterface = %q, want \"*\"", cfg.NetworkInterface)
	}
	if cfg.GPUMethod != "auto" {
		t.Errorf("GPUMethod = %q, want \"auto\"", cfg.GPUMethod)
	}
}

func TestNewCollector(t *testing.T) {
	t.Run("with nil config", func(t *testing.T) {
		c := NewCollector(nil)
		if c == nil {
			t.Fatal("NewCollector(nil) returned nil")
		}
		if c.config == nil {
			t.Error("config should not be nil")
		}
		if c.prevNetStats == nil {
			t.Error("prevNetStats should be initialized")
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		cfg := &Config{
			ShowCPU:    true,
			ShowGPU:    false,
			DiskMounts: []string{"/home"},
			GPUMethod:  "nvidia",
		}
		c := NewCollector(cfg)
		if c.config != cfg {
			t.Error("config should match provided config")
		}
	})
}

func TestCollector_Collect(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping on non-Linux platform")
	}

	cfg := &Config{
		ShowCPU:     true,
		ShowGPU:     false, // Skip GPU - may not be available
		ShowRAM:     true,
		ShowDisk:    true,
		ShowNetwork: true,
		DiskMounts:  []string{"/"},
	}

	c := NewCollector(cfg)
	data := c.Collect()

	if data == nil {
		t.Fatal("Collect() returned nil")
	}

	if data.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}

	// CPU should have at least CoreCount
	if data.CPU.CoreCount <= 0 {
		t.Errorf("CoreCount = %d, want > 0", data.CPU.CoreCount)
	}

	// Memory should have totals
	if data.Memory.TotalMB <= 0 {
		t.Errorf("Memory.TotalMB = %f, want > 0", data.Memory.TotalMB)
	}
}

func TestCollector_CollectWithDisabledSensors(t *testing.T) {
	cfg := &Config{
		ShowCPU:     false,
		ShowGPU:     false,
		ShowRAM:     false,
		ShowDisk:    false,
		ShowNetwork: false,
	}

	c := NewCollector(cfg)
	data := c.Collect()

	if data == nil {
		t.Fatal("Collect() returned nil")
	}

	// CPU should be zeroed (except for things that don't require proc)
	if data.CPU.LoadPercent != 0 {
		t.Errorf("CPU.LoadPercent should be 0 when disabled, got %f", data.CPU.LoadPercent)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes float64
		want  string
	}{
		{0, "0.0B"},
		{100, "100.0B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{1024 * 1024, "1.0MB"},
		{1024 * 1024 * 1024, "1.0GB"},
		{1024 * 1024 * 1024 * 1024, "1.0TB"},
		{1024 * 1024 * 1024 * 1024 * 1024, "1.0PB"},
		{1024 * 1024 * 1024 * 1024 * 1024 * 2, "2.0PB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatBytes(%f) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestFormatBytesPerSec(t *testing.T) {
	tests := []struct {
		bps  float64
		want string
	}{
		{0, "0.0B/s"},
		{1024, "1.0KB/s"},
		{1024 * 1024, "1.0MB/s"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := FormatBytesPerSec(tt.bps)
			if got != tt.want {
				t.Errorf("FormatBytesPerSec(%f) = %q, want %q", tt.bps, got, tt.want)
			}
		})
	}
}

func TestCPUStats_Fields(t *testing.T) {
	// Test that fields can be set correctly
	temp := 65.5
	freq := 3500.0

	stats := CPUStats{
		Temperature:  &temp,
		LoadPercent:  50.5,
		FrequencyMHz: &freq,
		CoreCount:    8,
	}

	if stats.CoreCount != 8 {
		t.Errorf("CoreCount = %d, want 8", stats.CoreCount)
	}
	if *stats.Temperature != 65.5 {
		t.Errorf("Temperature = %f, want 65.5", *stats.Temperature)
	}
	if stats.LoadPercent != 50.5 {
		t.Errorf("LoadPercent = %f, want 50.5", stats.LoadPercent)
	}
}

func TestGPUStats_Fields(t *testing.T) {
	temp := 70.0
	load := 80.0
	memUsed := 4096.0
	memTotal := 8192.0
	power := 200.0

	stats := GPUStats{
		Name:          "Test GPU",
		Temperature:   &temp,
		LoadPercent:   &load,
		MemoryUsedMB:  &memUsed,
		MemoryTotalMB: &memTotal,
		PowerWatts:    &power,
		Available:     true,
	}

	if stats.Name != "Test GPU" {
		t.Errorf("Name = %q, want Test GPU", stats.Name)
	}
	if !stats.Available {
		t.Error("Available should be true")
	}
}

func TestMemoryStats_Fields(t *testing.T) {
	stats := MemoryStats{
		TotalMB:     16384,
		UsedMB:      8192,
		AvailableMB: 8192,
		Percent:     50.0,
	}

	if stats.TotalMB != 16384 {
		t.Errorf("TotalMB = %f, want 16384", stats.TotalMB)
	}
	if stats.Percent != 50.0 {
		t.Errorf("Percent = %f, want 50.0", stats.Percent)
	}
}

func TestDiskStats_Fields(t *testing.T) {
	stats := DiskStats{
		MountPoint: "/",
		TotalGB:    500,
		UsedGB:     250,
		FreeGB:     250,
		Percent:    50.0,
	}

	if stats.MountPoint != "/" {
		t.Errorf("MountPoint = %q, want /", stats.MountPoint)
	}
}

func TestNetworkStats_Fields(t *testing.T) {
	stats := NetworkStats{
		Interface:     "eth0",
		RxBytesPerSec: 1000000,
		TxBytesPerSec: 500000,
		RxTotalBytes:  100000000,
		TxTotalBytes:  50000000,
	}

	if stats.Interface != "eth0" {
		t.Errorf("Interface = %q, want eth0", stats.Interface)
	}
}

func TestData_Fields(t *testing.T) {
	data := &Data{
		CPU: CPUStats{
			CoreCount:   4,
			LoadPercent: 25.0,
		},
		Memory: MemoryStats{
			TotalMB: 8192,
		},
		Disks: []DiskStats{
			{MountPoint: "/"},
		},
		Networks: []NetworkStats{
			{Interface: "eth0"},
		},
	}

	if data.CPU.CoreCount != 4 {
		t.Errorf("CPU.CoreCount = %d, want 4", data.CPU.CoreCount)
	}
	if len(data.Disks) != 1 {
		t.Errorf("Disks length = %d, want 1", len(data.Disks))
	}
	if len(data.Networks) != 1 {
		t.Errorf("Networks length = %d, want 1", len(data.Networks))
	}
}

// Linux-specific tests
func TestCollector_CollectCPU(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping on non-Linux platform")
	}

	c := NewCollector(DefaultConfig())
	stats := c.collectCPU()

	if stats.CoreCount <= 0 {
		t.Errorf("CoreCount = %d, want > 0", stats.CoreCount)
	}
}

func TestCollector_CollectMemory(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping on non-Linux platform")
	}

	c := NewCollector(DefaultConfig())
	stats := c.collectMemory()

	if stats.TotalMB <= 0 {
		t.Errorf("TotalMB = %f, want > 0", stats.TotalMB)
	}
	if stats.AvailableMB < 0 {
		t.Errorf("AvailableMB = %f, want >= 0", stats.AvailableMB)
	}
}

func TestCollector_CollectDisks(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping on non-Linux platform")
	}

	cfg := &Config{
		DiskMounts: []string{"/"},
	}
	c := NewCollector(cfg)
	disks := c.collectDisks()

	if len(disks) == 0 {
		t.Error("Expected at least one disk stat for /")
	}

	if len(disks) > 0 && disks[0].TotalGB <= 0 {
		t.Errorf("TotalGB = %f, want > 0", disks[0].TotalGB)
	}
}

func TestCollector_CollectNetwork(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping on non-Linux platform")
	}

	c := NewCollector(DefaultConfig())
	networks := c.collectNetwork()

	// Should have at least one network interface (not counting lo)
	// Note: This might fail in minimal containers
	t.Logf("Found %d network interfaces", len(networks))
}

func TestCollector_CalculateCPULoad(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping on non-Linux platform")
	}

	c := NewCollector(DefaultConfig())

	// First call initializes state
	load1, ok1 := c.calculateCPULoad()
	if !ok1 {
		t.Skip("Could not read CPU stats")
	}
	if load1 != 0 {
		t.Logf("First load sample: %f (expected 0 for initialization)", load1)
	}

	// Second call should compute actual load
	load2, ok2 := c.calculateCPULoad()
	if !ok2 {
		t.Error("Second calculateCPULoad() failed")
	}
	if load2 < 0 || load2 > 100 {
		t.Errorf("Load = %f, want 0-100", load2)
	}
}
