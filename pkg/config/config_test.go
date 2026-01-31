//go:build linux

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUSBDevice_String(t *testing.T) {
	tests := []struct {
		name   string
		device USBDevice
		want   string
	}{
		{
			name:   "without serial",
			device: USBDevice{VendorID: 0x1234, ProductID: 0x5678},
			want:   "1234:5678",
		},
		{
			name:   "with serial",
			device: USBDevice{VendorID: 0x1234, ProductID: 0x5678, Serial: "ABC123"},
			want:   "1234:5678 (S/N: ABC123)",
		},
		{
			name:   "zero device without serial",
			device: USBDevice{VendorID: 0, ProductID: 0},
			want:   "0000:0000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.device.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUSBDevice_IsZero(t *testing.T) {
	tests := []struct {
		name   string
		device USBDevice
		want   bool
	}{
		{
			name:   "zero device",
			device: USBDevice{},
			want:   true,
		},
		{
			name:   "non-zero VendorID",
			device: USBDevice{VendorID: 0x1234},
			want:   false,
		},
		{
			name:   "non-zero ProductID",
			device: USBDevice{ProductID: 0x5678},
			want:   false,
		},
		{
			name:   "both non-zero",
			device: USBDevice{VendorID: 0x1234, ProductID: 0x5678},
			want:   false,
		},
		{
			name:   "zero VID/PID with serial",
			device: USBDevice{Serial: "ABC123"},
			want:   true, // Still zero if VID/PID are zero
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.device.IsZero(); got != tt.want {
				t.Errorf("IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Device.VendorID != 0 || cfg.Device.ProductID != 0 {
		t.Error("Default config should have no device configured")
	}
	if cfg.Brightness != 7 {
		t.Errorf("Brightness = %d, want 7", cfg.Brightness)
	}
	if cfg.UpdateInterval != 1.0 {
		t.Errorf("UpdateInterval = %f, want 1.0", cfg.UpdateInterval)
	}
	if len(cfg.DiskMounts) != 1 || cfg.DiskMounts[0] != "/" {
		t.Errorf("DiskMounts = %v, want [\"/\"]", cfg.DiskMounts)
	}
}

func TestConfigPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error = %v", err)
	}

	expected := filepath.Join(tmpDir, "sensorpanel", "config.json")
	if path != expected {
		t.Errorf("ConfigPath() = %q, want %q", path, expected)
	}
}

func TestConfigPath_DefaultDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	path, err := ConfigPath()
	if err != nil {
		t.Fatalf("ConfigPath() error = %v", err)
	}

	// Should end with .config/sensorpanel/config.json
	if !strings.HasSuffix(path, filepath.Join(".config", "sensorpanel", "config.json")) {
		t.Errorf("ConfigPath() = %q, want suffix ending with .config/sensorpanel/config.json", path)
	}
}

func TestLoad_DefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// No config file exists, should return default
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Brightness != 7 {
		t.Errorf("Brightness = %d, want 7", cfg.Brightness)
	}
	if cfg.Device.VendorID != 0 {
		t.Error("Device should not be configured")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg := &Config{
		Device: USBDevice{
			VendorID:  0x1234,
			ProductID: 0x5678,
			Serial:    "TEST123",
		},
		ProfileID:      "qtkeji",
		Brightness:     5,
		Theme:          "my-theme",
		UpdateInterval: 2.5,
		DiskMounts:     []string{"/", "/home"},
	}

	// Save
	err := Save(cfg)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(tmpDir, "sensorpanel", "config.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("Config file not created: %v", err)
	}

	// Load
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify values
	if loaded.Device.VendorID != 0x1234 {
		t.Errorf("VendorID = 0x%04X, want 0x1234", loaded.Device.VendorID)
	}
	if loaded.Device.ProductID != 0x5678 {
		t.Errorf("ProductID = 0x%04X, want 0x5678", loaded.Device.ProductID)
	}
	if loaded.Device.Serial != "TEST123" {
		t.Errorf("Serial = %q, want TEST123", loaded.Device.Serial)
	}
	if loaded.ProfileID != "qtkeji" {
		t.Errorf("ProfileID = %q, want qtkeji", loaded.ProfileID)
	}
	if loaded.Brightness != 5 {
		t.Errorf("Brightness = %d, want 5", loaded.Brightness)
	}
	if loaded.Theme != "my-theme" {
		t.Errorf("Theme = %q, want my-theme", loaded.Theme)
	}
	if loaded.UpdateInterval != 2.5 {
		t.Errorf("UpdateInterval = %f, want 2.5", loaded.UpdateInterval)
	}
	if len(loaded.DiskMounts) != 2 {
		t.Errorf("DiskMounts length = %d, want 2", len(loaded.DiskMounts))
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Create config directory and invalid JSON file
	configDir := filepath.Join(tmpDir, "sensorpanel")
	os.MkdirAll(configDir, 0755)
	configPath := filepath.Join(configDir, "config.json")
	os.WriteFile(configPath, []byte("{ invalid json"), 0644)

	_, err := Load()
	if err == nil {
		t.Error("Load() expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "parse config") {
		t.Errorf("Error should contain 'parse config', got: %v", err)
	}
}

func TestSetDevice(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	err := SetDevice(0xABCD, 0xEF01, "SERIAL123")
	if err != nil {
		t.Fatalf("SetDevice() error = %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Device.VendorID != 0xABCD {
		t.Errorf("VendorID = 0x%04X, want 0xABCD", cfg.Device.VendorID)
	}
	if cfg.Device.ProductID != 0xEF01 {
		t.Errorf("ProductID = 0x%04X, want 0xEF01", cfg.Device.ProductID)
	}
	if cfg.Device.Serial != "SERIAL123" {
		t.Errorf("Serial = %q, want SERIAL123", cfg.Device.Serial)
	}
}

func TestGetDevice(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// No config, should return zero device
	device, err := GetDevice()
	if err != nil {
		t.Fatalf("GetDevice() error = %v", err)
	}
	if !device.IsZero() {
		t.Error("Expected zero device when no config exists")
	}

	// Set device and retrieve
	SetDevice(0x1234, 0x5678, "")
	device, err = GetDevice()
	if err != nil {
		t.Fatalf("GetDevice() error = %v", err)
	}
	if device.VendorID != 0x1234 || device.ProductID != 0x5678 {
		t.Errorf("Device = %v, want 1234:5678", device)
	}
}

func TestSetTheme(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	err := SetTheme("cool-theme")
	if err != nil {
		t.Fatalf("SetTheme() error = %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Theme != "cool-theme" {
		t.Errorf("Theme = %q, want cool-theme", cfg.Theme)
	}
}

func TestGetTheme(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// No config, should return empty
	theme, err := GetTheme()
	if err != nil {
		t.Fatalf("GetTheme() error = %v", err)
	}
	if theme != "" {
		t.Errorf("Theme = %q, want empty string", theme)
	}

	// Set theme and retrieve
	SetTheme("my-theme")
	theme, err = GetTheme()
	if err != nil {
		t.Fatalf("GetTheme() error = %v", err)
	}
	if theme != "my-theme" {
		t.Errorf("Theme = %q, want my-theme", theme)
	}
}

func TestSetProfileID(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	err := SetProfileID("custom_profile")
	if err != nil {
		t.Fatalf("SetProfileID() error = %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ProfileID != "custom_profile" {
		t.Errorf("ProfileID = %q, want custom_profile", cfg.ProfileID)
	}
}

func TestGetProfileID(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// No config, should return empty
	profileID, err := GetProfileID()
	if err != nil {
		t.Fatalf("GetProfileID() error = %v", err)
	}
	if profileID != "" {
		t.Errorf("ProfileID = %q, want empty string", profileID)
	}

	// Set profile and retrieve
	SetProfileID("qtkeji")
	profileID, err = GetProfileID()
	if err != nil {
		t.Fatalf("GetProfileID() error = %v", err)
	}
	if profileID != "qtkeji" {
		t.Errorf("ProfileID = %q, want qtkeji", profileID)
	}
}

func TestSetDevice_PreservesOtherSettings(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// First set a theme
	SetTheme("original-theme")
	SetProfileID("original-profile")

	// Then set device
	SetDevice(0x1234, 0x5678, "")

	// Theme and profile should be preserved
	cfg, _ := Load()
	if cfg.Theme != "original-theme" {
		t.Errorf("Theme = %q, want original-theme", cfg.Theme)
	}
	if cfg.ProfileID != "original-profile" {
		t.Errorf("ProfileID = %q, want original-profile", cfg.ProfileID)
	}
	if cfg.Device.VendorID != 0x1234 {
		t.Errorf("VendorID = 0x%04X, want 0x1234", cfg.Device.VendorID)
	}
}

func TestConfigCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	// Directory doesn't exist yet
	configDir := filepath.Join(tmpDir, "sensorpanel")
	if _, err := os.Stat(configDir); !os.IsNotExist(err) {
		t.Skip("Directory already exists")
	}

	// Save should create directory
	err := Save(DefaultConfig())
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Directory should now exist
	info, err := os.Stat(configDir)
	if err != nil {
		t.Fatalf("Config directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Config path is not a directory")
	}
}

// Tests for discovery.go

func TestDiscoveredDevice_String(t *testing.T) {
	tests := []struct {
		name   string
		device DiscoveredDevice
		want   string
	}{
		{
			name: "VID/PID only",
			device: DiscoveredDevice{
				VendorID:  0x1234,
				ProductID: 0x5678,
			},
			want: "1234:5678",
		},
		{
			name: "with manufacturer and product",
			device: DiscoveredDevice{
				VendorID:     0x1234,
				ProductID:    0x5678,
				Manufacturer: "TestCo",
				Product:      "TestDevice",
			},
			want: "1234:5678 - TestCo TestDevice",
		},
		{
			name: "with serial",
			device: DiscoveredDevice{
				VendorID:  0x1234,
				ProductID: 0x5678,
				Serial:    "ABC123",
			},
			want: "1234:5678 - S/N:ABC123",
		},
		{
			name: "with manufacturer only",
			device: DiscoveredDevice{
				VendorID:     0x1234,
				ProductID:    0x5678,
				Manufacturer: "TestCo",
			},
			want: "1234:5678 - TestCo",
		},
		{
			name: "with all fields",
			device: DiscoveredDevice{
				VendorID:     0x1234,
				ProductID:    0x5678,
				Manufacturer: "TestCo",
				Product:      "TestDevice",
				Serial:       "ABC123",
			},
			want: "1234:5678 - TestCo TestDevice - S/N:ABC123",
		},
		{
			name: "empty manufacturer and product",
			device: DiscoveredDevice{
				VendorID:     0x1234,
				ProductID:    0x5678,
				Manufacturer: "",
				Product:      "",
			},
			want: "1234:5678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.device.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDiscoveredDevice_ToUSBDevice(t *testing.T) {
	discovered := DiscoveredDevice{
		VendorID:     0x1234,
		ProductID:    0x5678,
		Manufacturer: "TestCo",
		Product:      "TestDevice",
		Serial:       "ABC123",
		Speed:        "high",
		BusAddr:      "1:2",
		IsProbable:   true,
	}

	usbDevice := discovered.ToUSBDevice()

	if usbDevice.VendorID != 0x1234 {
		t.Errorf("VendorID = 0x%04X, want 0x1234", usbDevice.VendorID)
	}
	if usbDevice.ProductID != 0x5678 {
		t.Errorf("ProductID = 0x%04X, want 0x5678", usbDevice.ProductID)
	}
	if usbDevice.Serial != "ABC123" {
		t.Errorf("Serial = %q, want ABC123", usbDevice.Serial)
	}
}

func TestIsKnownDisplay(t *testing.T) {
	// Test with known QTKeJi VID/PID
	if !isKnownDisplay(0x1908, 0x0102) {
		t.Error("isKnownDisplay(0x1908, 0x0102) = false, want true for QTKeJi")
	}

	// Test with unknown device
	if isKnownDisplay(0x9999, 0x9999) {
		t.Error("isKnownDisplay(0x9999, 0x9999) = true, want false for unknown device")
	}
}
