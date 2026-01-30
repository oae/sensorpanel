package device

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"unicode"
)

// DeviceSpec contains the specification for generating a device profile.
type DeviceSpec struct {
	ID            string
	Name          string
	Description   string
	VendorID      uint16
	ProductID     uint16
	Width         int
	Height        int
	ColorFormat   ColorFormat
	ByteOrder     ByteOrder
	MaxBrightness int
	ProtocolType  ProtocolType
}

// Validate checks if the spec has all required fields.
func (s *DeviceSpec) Validate() error {
	if s.ID == "" {
		return fmt.Errorf("ID is required")
	}
	if !isValidIdentifier(s.ID) {
		return fmt.Errorf("ID must be a valid Go identifier (lowercase, no spaces)")
	}
	if s.Name == "" {
		return fmt.Errorf("Name is required")
	}
	if s.VendorID == 0 {
		return fmt.Errorf("VendorID is required")
	}
	if s.ProductID == 0 {
		return fmt.Errorf("ProductID is required")
	}
	if s.Width <= 0 {
		return fmt.Errorf("Width must be positive")
	}
	if s.Height <= 0 {
		return fmt.Errorf("Height must be positive")
	}
	return nil
}

// StructName returns the Go struct name for this device.
func (s *DeviceSpec) StructName() string {
	// Convert ID to PascalCase and add "Profile" suffix
	return toPascalCase(s.ID) + "Profile"
}

// ColorFormatStr returns the ColorFormat as a Go constant string.
func (s *DeviceSpec) ColorFormatStr() string {
	switch s.ColorFormat {
	case RGB565:
		return "RGB565"
	case RGB888:
		return "RGB888"
	default:
		return "RGB565"
	}
}

// ByteOrderStr returns the ByteOrder as a Go constant string.
func (s *DeviceSpec) ByteOrderStr() string {
	switch s.ByteOrder {
	case BigEndian:
		return "BigEndian"
	case LittleEndian:
		return "LittleEndian"
	default:
		return "BigEndian"
	}
}

// ProtocolTypeStr returns the ProtocolType as a Go constant string.
func (s *DeviceSpec) ProtocolTypeStr() string {
	switch s.ProtocolType {
	case ProtocolSCSI:
		return "ProtocolSCSI"
	case ProtocolBulk:
		return "ProtocolBulk"
	default:
		return "ProtocolSCSI"
	}
}

// BufferSize calculates the buffer size for the display.
func (s *DeviceSpec) BufferSize() int {
	return s.Width * s.Height * s.ColorFormat.BytesPerPixel()
}

// GenerateProfile generates Go source code for a device profile.
func GenerateProfile(spec DeviceSpec) (string, error) {
	if err := spec.Validate(); err != nil {
		return "", fmt.Errorf("invalid spec: %w", err)
	}

	tmpl, err := template.New("profile").Parse(profileTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, &spec); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// isValidIdentifier checks if s is a valid Go identifier (lowercase).
func isValidIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
		} else {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
				return false
			}
		}
	}
	// Should be lowercase
	return s == strings.ToLower(s)
}

// toPascalCase converts a snake_case or kebab-case string to PascalCase.
func toPascalCase(s string) string {
	var result strings.Builder
	capitalizeNext := true

	for _, r := range s {
		if r == '_' || r == '-' {
			capitalizeNext = true
			continue
		}
		if capitalizeNext {
			result.WriteRune(unicode.ToUpper(r))
			capitalizeNext = false
		} else {
			result.WriteRune(r)
		}
	}

	return result.String()
}

const profileTemplate = `package device

import (
	"image"

	"github.com/alperen/sensorpanel/pkg/panel"
)

// {{.StructName}} implements DeviceProfile for {{.Name}}.
//
// Device Information:
//   - Resolution: {{.Width}}x{{.Height}}
//   - Color Format: {{.ColorFormatStr}}
//   - Byte Order: {{.ByteOrderStr}}
//   - VID:PID: 0x{{printf "%04X" .VendorID}}:0x{{printf "%04X" .ProductID}}
//
// TODO: Implement the protocol methods based on USB traffic analysis.
type {{.StructName}} struct{}

// ID returns the unique identifier for this profile.
func (p *{{.StructName}}) ID() string {
	return "{{.ID}}"
}

// Name returns the human-readable name.
func (p *{{.StructName}}) Name() string {
	return "{{.Name}}"
}

// Description returns a brief description.
func (p *{{.StructName}}) Description() string {
	return "{{.Description}}"
}

// Matches returns true if this profile supports the given VID/PID.
func (p *{{.StructName}}) Matches(vendorID, productID uint16) bool {
	return vendorID == 0x{{printf "%04X" .VendorID}} && productID == 0x{{printf "%04X" .ProductID}}
}

// VendorIDs returns all vendor IDs this profile supports.
func (p *{{.StructName}}) VendorIDs() []uint16 {
	return []uint16{0x{{printf "%04X" .VendorID}}}
}

// ProductIDs returns all product IDs this profile supports.
func (p *{{.StructName}}) ProductIDs() []uint16 {
	return []uint16{0x{{printf "%04X" .ProductID}}}
}

// Width returns the display width in pixels.
func (p *{{.StructName}}) Width() int {
	return {{.Width}}
}

// Height returns the display height in pixels.
func (p *{{.StructName}}) Height() int {
	return {{.Height}}
}

// ColorFormat returns the pixel color format.
func (p *{{.StructName}}) ColorFormat() ColorFormat {
	return {{.ColorFormatStr}}
}

// ByteOrder returns the byte order for pixel data.
func (p *{{.StructName}}) ByteOrder() ByteOrder {
	return {{.ByteOrderStr}}
}

// BufferSize returns the total frame buffer size in bytes.
func (p *{{.StructName}}) BufferSize() int {
	return {{.BufferSize}}
}

// MaxBrightness returns the maximum brightness level.
func (p *{{.StructName}}) MaxBrightness() int {
	return {{.MaxBrightness}}
}

// ProtocolType returns the USB protocol type.
func (p *{{.StructName}}) ProtocolType() ProtocolType {
	return {{.ProtocolTypeStr}}
}

// BlitCommand builds the command to send image data.
//
// TODO: Implement based on your device's protocol.
// This is the most important method - it creates the USB command
// that tells the device where to draw the image data.
//
// For SCSI-based devices, you typically need to:
//   1. Create a 16-byte CDB (Command Descriptor Block)
//   2. Wrap it in a CBW (Command Block Wrapper) using panel.BuildCBW()
//
// Example for a typical vendor command:
//   cdb := make([]byte, 16)
//   cdb[0] = 0xCD  // Vendor command prefix
//   cdb[1] = 0x00  // Sub-command for blit
//   // ... encode x, y, width, height in remaining bytes
//   return panel.BuildCBW(cdb, dataLen, panel.DirOut)
func (p *{{.StructName}}) BlitCommand(x, y, w, h int, dataLen int) []byte {
	// TODO: Implement based on USB traffic analysis
	//
	// Steps to figure out your device's protocol:
	// 1. Use Wireshark with USBPcap to capture traffic from manufacturer software
	// 2. Look for bulk OUT transfers after the display updates
	// 3. The first 31 bytes are usually the CBW (SCSI Command Block Wrapper)
	// 4. Bytes 15-30 of the CBW contain the CDB (Command Descriptor Block)
	// 5. Identify how x, y, width, height are encoded
	//
	// For now, this returns a placeholder that will fail gracefully
	panic("BlitCommand not implemented for {{.ID}} - see comments for implementation guide")
}

// BacklightCommand builds the command to set backlight brightness.
//
// TODO: Implement based on your device's protocol.
// Many devices use a simple vendor command with the brightness level.
func (p *{{.StructName}}) BacklightCommand(level int) []byte {
	// TODO: Implement based on USB traffic analysis
	//
	// For SCSI-based devices, this is typically:
	//   cdb := make([]byte, 16)
	//   cdb[0] = 0xCD  // Vendor command prefix
	//   cdb[1] = 0x07  // Backlight sub-command (varies by device)
	//   cdb[2] = byte(level)
	//   return panel.BuildCBW(cdb, 0, panel.DirNone)
	//
	// For now, this returns a placeholder
	panic("BacklightCommand not implemented for {{.ID}} - see comments for implementation guide")
}

// ParseResponse validates a response from the device.
func (p *{{.StructName}}) ParseResponse(data []byte) error {
	// Most devices use standard SCSI CSW responses
	// The default implementation should work for most cases
	return nil
}

// ConvertImage converts a Go image to the device's native pixel format.
func (p *{{.StructName}}) ConvertImage(img image.Image) []byte {
	// Use the panel package's conversion functions based on color format and byte order
	{{- if eq .ColorFormatStr "RGB565"}}
	{{- if eq .ByteOrderStr "BigEndian"}}
	return panel.ImageToRGB565BufferBE(img)
	{{- else}}
	return panel.ImageToRGB565BufferLE(img)
	{{- end}}
	{{- else}}
	return panel.ImageToRGB888Buffer(img)
	{{- end}}
}
`
