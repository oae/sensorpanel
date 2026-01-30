// Package device provides device profile abstraction for different USB display panels.
//
// Each supported device type implements the DeviceProfile interface, which defines
// the device's properties (resolution, color format) and protocol (command building).
package device

import "image"

// ColorFormat represents the pixel color format used by the device.
type ColorFormat int

const (
	// RGB565 is 16-bit color (5 bits red, 6 bits green, 5 bits blue).
	RGB565 ColorFormat = iota
	// RGB888 is 24-bit color (8 bits per channel).
	RGB888
)

func (c ColorFormat) String() string {
	switch c {
	case RGB565:
		return "RGB565"
	case RGB888:
		return "RGB888"
	default:
		return "Unknown"
	}
}

// BytesPerPixel returns the number of bytes per pixel for this color format.
func (c ColorFormat) BytesPerPixel() int {
	switch c {
	case RGB565:
		return 2
	case RGB888:
		return 3
	default:
		return 2
	}
}

// ByteOrder represents the byte order for multi-byte pixel values.
type ByteOrder int

const (
	// BigEndian means most significant byte first.
	BigEndian ByteOrder = iota
	// LittleEndian means least significant byte first.
	LittleEndian
)

func (b ByteOrder) String() string {
	switch b {
	case BigEndian:
		return "big-endian"
	case LittleEndian:
		return "little-endian"
	default:
		return "unknown"
	}
}

// ProtocolType represents the USB protocol used by the device.
type ProtocolType int

const (
	// ProtocolSCSI uses SCSI Command Block Wrapper for commands.
	ProtocolSCSI ProtocolType = iota
	// ProtocolBulk uses raw bulk transfers.
	ProtocolBulk
)

func (p ProtocolType) String() string {
	switch p {
	case ProtocolSCSI:
		return "SCSI"
	case ProtocolBulk:
		return "Bulk"
	default:
		return "Unknown"
	}
}

// DeviceProfile defines the interface for a USB display device profile.
//
// Each supported device type implements this interface to provide:
// - Device identification (VID/PID matching)
// - Display properties (resolution, color format)
// - Protocol implementation (command building, image conversion)
type DeviceProfile interface {
	// Identity

	// ID returns the unique identifier for this profile (e.g., "qtkeji", "ax206").
	ID() string

	// Name returns the human-readable name of the device.
	Name() string

	// Description returns a brief description of the device.
	Description() string

	// Matches returns true if this profile supports the given VID/PID.
	Matches(vendorID, productID uint16) bool

	// VendorIDs returns all vendor IDs this profile supports.
	VendorIDs() []uint16

	// ProductIDs returns all product IDs this profile supports.
	ProductIDs() []uint16

	// Display Properties

	// Width returns the display width in pixels.
	Width() int

	// Height returns the display height in pixels.
	Height() int

	// ColorFormat returns the pixel color format.
	ColorFormat() ColorFormat

	// ByteOrder returns the byte order for pixel data.
	ByteOrder() ByteOrder

	// BufferSize returns the total frame buffer size in bytes.
	BufferSize() int

	// Backlight

	// MaxBrightness returns the maximum brightness level (0 = no backlight control).
	MaxBrightness() int

	// Protocol

	// ProtocolType returns the USB protocol type used by this device.
	ProtocolType() ProtocolType

	// BlitCommand builds the command bytes for sending image data to the display.
	// Parameters specify the destination rectangle and data length.
	BlitCommand(x, y, w, h int, dataLen int) []byte

	// BacklightCommand builds the command bytes for setting backlight brightness.
	BacklightCommand(level int) []byte

	// ParseResponse validates a response from the device.
	// Returns nil if the response indicates success, error otherwise.
	ParseResponse(data []byte) error

	// Image Conversion

	// ConvertImage converts a Go image to the device's native pixel format.
	// The returned byte slice is ready to be sent to the device.
	ConvertImage(img image.Image) []byte
}

// ProfileInfo contains basic information about a device profile for display purposes.
type ProfileInfo struct {
	ID          string
	Name        string
	Description string
	Width       int
	Height      int
	ColorFormat ColorFormat
	ByteOrder   ByteOrder
	VendorIDs   []uint16
	ProductIDs  []uint16
}

// GetInfo extracts displayable information from a DeviceProfile.
func GetInfo(p DeviceProfile) ProfileInfo {
	return ProfileInfo{
		ID:          p.ID(),
		Name:        p.Name(),
		Description: p.Description(),
		Width:       p.Width(),
		Height:      p.Height(),
		ColorFormat: p.ColorFormat(),
		ByteOrder:   p.ByteOrder(),
		VendorIDs:   p.VendorIDs(),
		ProductIDs:  p.ProductIDs(),
	}
}
