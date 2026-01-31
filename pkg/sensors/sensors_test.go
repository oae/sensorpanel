package sensors

import (
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
