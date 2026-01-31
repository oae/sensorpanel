// Package config handles platform-specific configuration for sensorpanel.
//
// Configuration is stored in platform-specific locations:
//   - Linux:   $XDG_CONFIG_HOME/sensorpanel/config.json (default: ~/.config/sensorpanel/)
//   - macOS:   ~/Library/Application Support/sensorpanel/config.json
//   - Windows: %APPDATA%\sensorpanel\config.json
//
// The config file stores:
//   - Selected USB device (VID, PID, Serial)
//   - Display settings (brightness, etc.)
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alperen/sensorpanel/pkg/paths"
)

const (
	appName    = "sensorpanel"
	configFile = "config.json"
)

// USBDevice represents a USB device identifier.
type USBDevice struct {
	VendorID  uint16 `json:"vendor_id"`
	ProductID uint16 `json:"product_id"`
	Serial    string `json:"serial,omitempty"` // Optional: disambiguate multiple identical devices
}

// String returns a human-readable representation.
func (d USBDevice) String() string {
	if d.Serial != "" {
		return fmt.Sprintf("%04x:%04x (S/N: %s)", d.VendorID, d.ProductID, d.Serial)
	}
	return fmt.Sprintf("%04x:%04x", d.VendorID, d.ProductID)
}

// IsZero returns true if the device is not configured.
func (d USBDevice) IsZero() bool {
	return d.VendorID == 0 && d.ProductID == 0
}

// Config represents the application configuration.
type Config struct {
	// Selected USB display device
	Device USBDevice `json:"device"`

	// Device profile ID (e.g., "qtkeji", "generic")
	// If empty, profile is auto-detected from VID/PID
	ProfileID string `json:"profile_id,omitempty"`

	// Display settings
	Brightness int `json:"brightness,omitempty"` // 0-7, default 7

	// Theme settings
	Theme string `json:"theme,omitempty"` // Active theme name (empty = use built-in renderer)

	// Sensor settings
	UpdateInterval float64  `json:"update_interval,omitempty"` // seconds
	DiskMounts     []string `json:"disk_mounts,omitempty"`
}

// DefaultConfig returns a config with sensible defaults.
// Note: Device is not set - user must select a device first.
func DefaultConfig() *Config {
	return &Config{
		// Device is intentionally empty - must be configured via 'device select'
		Brightness:     7,
		UpdateInterval: 1.0,
		DiskMounts:     paths.DefaultDiskMounts(),
	}
}

// configDir returns the platform-specific config directory for sensorpanel.
func configDir() (string, error) {
	return paths.ConfigDir()
}

// ConfigPath returns the full path to the config file.
func ConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFile), nil
}

// Load reads the config from the XDG config file.
// Returns default config if file doesn't exist.
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}

// Save writes the config to the XDG config file.
func Save(cfg *Config) error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	// Create config directory if needed
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	path := filepath.Join(dir, configFile)

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// SetDevice updates the selected device and saves the config.
func SetDevice(vid, pid uint16, serial string) error {
	cfg, err := Load()
	if err != nil {
		cfg = DefaultConfig()
	}

	cfg.Device = USBDevice{
		VendorID:  vid,
		ProductID: pid,
		Serial:    serial,
	}

	return Save(cfg)
}

// GetDevice returns the currently configured device.
func GetDevice() (USBDevice, error) {
	cfg, err := Load()
	if err != nil {
		return USBDevice{}, err
	}
	return cfg.Device, nil
}

// SetTheme updates the active theme and saves the config.
func SetTheme(themeName string) error {
	cfg, err := Load()
	if err != nil {
		cfg = DefaultConfig()
	}

	cfg.Theme = themeName
	return Save(cfg)
}

// GetTheme returns the currently configured theme name.
func GetTheme() (string, error) {
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	return cfg.Theme, nil
}

// SetProfileID updates the device profile ID and saves the config.
func SetProfileID(profileID string) error {
	cfg, err := Load()
	if err != nil {
		cfg = DefaultConfig()
	}

	cfg.ProfileID = profileID
	return Save(cfg)
}

// GetProfileID returns the currently configured device profile ID.
func GetProfileID() (string, error) {
	cfg, err := Load()
	if err != nil {
		return "", err
	}
	return cfg.ProfileID, nil
}
