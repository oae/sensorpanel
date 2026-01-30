// Package config provides USB device discovery for sensorpanel.
package config

import (
	"fmt"
	"strings"

	"github.com/alperen/sensorpanel/pkg/device"
	"github.com/google/gousb"
)

// DiscoveredDevice represents a USB device found during scanning.
type DiscoveredDevice struct {
	VendorID     uint16
	ProductID    uint16
	Manufacturer string
	Product      string
	Serial       string
	Speed        string
	BusAddr      string // Bus:Address for identification
	IsProbable   bool   // Likely to be a display panel based on heuristics
}

// String returns a human-readable description.
func (d DiscoveredDevice) String() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%04x:%04x", d.VendorID, d.ProductID))

	if d.Manufacturer != "" || d.Product != "" {
		name := strings.TrimSpace(d.Manufacturer + " " + d.Product)
		if name != "" {
			parts = append(parts, name)
		}
	}

	if d.Serial != "" {
		parts = append(parts, fmt.Sprintf("S/N:%s", d.Serial))
	}

	return strings.Join(parts, " - ")
}

// ToUSBDevice converts to a USBDevice for config storage.
func (d DiscoveredDevice) ToUSBDevice() USBDevice {
	return USBDevice{
		VendorID:  d.VendorID,
		ProductID: d.ProductID,
		Serial:    d.Serial,
	}
}

// isKnownDisplay checks if a device matches known display panels from the device registry.
func isKnownDisplay(vid, pid uint16) bool {
	return device.IsKnownDevice(vid, pid)
}

// isProbableDisplay uses heuristics to guess if a device might be a display.
func isProbableDisplay(desc *gousb.DeviceDesc, manufacturer, product string) bool {
	// Check known devices first
	if isKnownDisplay(uint16(desc.Vendor), uint16(desc.Product)) {
		return true
	}

	// Heuristics for unknown devices:
	// 1. Product name contains display-related keywords
	productLower := strings.ToLower(product)
	manufacturerLower := strings.ToLower(manufacturer)

	displayKeywords := []string{"display", "lcd", "screen", "panel", "aida64", "monitor"}
	for _, kw := range displayKeywords {
		if strings.Contains(productLower, kw) || strings.Contains(manufacturerLower, kw) {
			return true
		}
	}

	// 2. Has bulk endpoints (required for image transfer)
	hasBulkOut := false
	hasBulkIn := false
	for _, cfg := range desc.Configs {
		for _, intf := range cfg.Interfaces {
			for _, alt := range intf.AltSettings {
				for _, ep := range alt.Endpoints {
					if ep.TransferType == gousb.TransferTypeBulk {
						if ep.Direction == gousb.EndpointDirectionOut {
							hasBulkOut = true
						} else {
							hasBulkIn = true
						}
					}
				}
			}
		}
	}

	// Device with bulk endpoints and vendor ID 0x1908 is very likely
	if desc.Vendor == 0x1908 && hasBulkOut && hasBulkIn {
		return true
	}

	return false
}

// DiscoverDevices scans USB bus for potential display devices.
// If knownOnly is true, only returns devices matching KnownDisplayVendors.
// Otherwise, uses heuristics to find probable display devices.
func DiscoverDevices(knownOnly bool) ([]DiscoveredDevice, error) {
	ctx := gousb.NewContext()
	defer ctx.Close()

	var discovered []DiscoveredDevice

	// Enumerate all USB devices
	devices, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		// Open all devices to inspect them
		return true
	})
	if err != nil {
		// Partial errors are common (permission denied, etc.)
		// Continue with devices we could open
	}

	for _, dev := range devices {
		defer dev.Close()

		manufacturer, _ := dev.Manufacturer()
		product, _ := dev.Product()
		serial, _ := dev.SerialNumber()

		isProbable := isProbableDisplay(dev.Desc, manufacturer, product)

		if knownOnly && !isKnownDisplay(uint16(dev.Desc.Vendor), uint16(dev.Desc.Product)) {
			continue
		}

		if !knownOnly || isProbable {
			discovered = append(discovered, DiscoveredDevice{
				VendorID:     uint16(dev.Desc.Vendor),
				ProductID:    uint16(dev.Desc.Product),
				Manufacturer: manufacturer,
				Product:      product,
				Serial:       serial,
				Speed:        dev.Desc.Speed.String(),
				BusAddr:      fmt.Sprintf("%d:%d", dev.Desc.Bus, dev.Desc.Address),
				IsProbable:   isProbable,
			})
		}
	}

	return discovered, nil
}

// FindConfiguredDevice attempts to find and validate the configured device.
// Returns the discovered device info if found, or an error if not found.
func FindConfiguredDevice() (*DiscoveredDevice, error) {
	cfg, err := Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.Device.IsZero() {
		return nil, fmt.Errorf("no device configured")
	}

	ctx := gousb.NewContext()
	defer ctx.Close()

	devices, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		return uint16(desc.Vendor) == cfg.Device.VendorID &&
			uint16(desc.Product) == cfg.Device.ProductID
	})
	if err != nil {
		// Partial errors are OK
	}

	if len(devices) == 0 {
		return nil, fmt.Errorf("configured device %s not found", cfg.Device)
	}

	// If serial is specified, find matching device
	for _, dev := range devices {
		defer dev.Close()

		serial, _ := dev.SerialNumber()

		// If config has serial, match it; otherwise take first match
		if cfg.Device.Serial != "" && serial != cfg.Device.Serial {
			continue
		}

		manufacturer, _ := dev.Manufacturer()
		product, _ := dev.Product()

		return &DiscoveredDevice{
			VendorID:     uint16(dev.Desc.Vendor),
			ProductID:    uint16(dev.Desc.Product),
			Manufacturer: manufacturer,
			Product:      product,
			Serial:       serial,
			Speed:        dev.Desc.Speed.String(),
			BusAddr:      fmt.Sprintf("%d:%d", dev.Desc.Bus, dev.Desc.Address),
			IsProbable:   true,
		}, nil
	}

	// Close remaining devices
	for _, dev := range devices {
		dev.Close()
	}

	return nil, fmt.Errorf("configured device with serial %s not found", cfg.Device.Serial)
}

// AutoDetectOrPrompt tries to find a display device automatically.
// Returns the device to use, whether user interaction was needed, and any error.
func AutoDetectOrPrompt() (*DiscoveredDevice, bool, error) {
	// First, try to find configured device
	if dev, err := FindConfiguredDevice(); err == nil {
		return dev, false, nil
	}

	// Try to find known devices
	known, err := DiscoverDevices(true)
	if err == nil && len(known) == 1 {
		// Exactly one known device found - use it
		return &known[0], false, nil
	}

	if err == nil && len(known) > 1 {
		// Multiple known devices - need user selection
		return nil, true, fmt.Errorf("multiple display devices found, please select one")
	}

	// No known devices - try heuristics
	probable, err := DiscoverDevices(false)
	if err != nil {
		return nil, false, fmt.Errorf("failed to scan USB devices: %w", err)
	}

	// Filter to only probable displays
	var displays []DiscoveredDevice
	for _, d := range probable {
		if d.IsProbable {
			displays = append(displays, d)
		}
	}

	if len(displays) == 0 {
		return nil, true, fmt.Errorf("no display devices found")
	}

	if len(displays) == 1 {
		return &displays[0], false, nil
	}

	// Multiple probable devices - need user selection
	return nil, true, fmt.Errorf("multiple potential display devices found, please select one")
}
