package sensors

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// EnabledSensors should be nil (all sensors enabled)
	if cfg.EnabledSensors != nil {
		t.Error("EnabledSensors should be nil by default (all sensors enabled)")
	}
	if cfg.DisabledSensors != nil {
		t.Error("DisabledSensors should be nil by default")
	}
	if cfg.Options != nil {
		t.Error("Options should be nil by default (providers use their defaults)")
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
