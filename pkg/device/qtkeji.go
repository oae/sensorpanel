package device

import (
	"encoding/binary"
	"fmt"
	"image"
)

// QTKeJi device constants
const (
	qtkejiWidth         = 480
	qtkejiHeight        = 320
	qtkejiMaxBrightness = 7

	// SCSI CBW constants
	cbwSignature = "USBC"
	cbwLength    = 31
	cswSignature = "USBS"
	cswLength    = 13

	// Direction flags
	dirOut = 0x00

	// Vendor command constants
	vendorCmdPrefix = 0xCD
	subcmdBlit      = 0x06
	blitCmdSetProp  = 0x01
	blitCmdWrite    = 0x12

	// Default tag for commands
	defaultTag = 0xDEADBEEF
)

// QTKeJiProfile implements DeviceProfile for QTKeJi/AIDA64 USB displays.
//
// These are 480x320 displays commonly sold as "AIDA64-compatible" USB sensor panels.
// They use SCSI-style vendor commands with RGB565 big-endian pixel format.
//
// Known VID/PID pairs:
//   - 1908:0102 (most common)
//   - 1908:0103 (variant)
type QTKeJiProfile struct{}

// ID returns the unique identifier for this profile.
func (p *QTKeJiProfile) ID() string {
	return "qtkeji"
}

// Name returns the human-readable device name.
func (p *QTKeJiProfile) Name() string {
	return "QTKeJi USB Display"
}

// Description returns a brief description.
func (p *QTKeJiProfile) Description() string {
	return "480x320 AIDA64-compatible USB sensor panel"
}

// Matches returns true if this profile supports the given VID/PID.
func (p *QTKeJiProfile) Matches(vendorID, productID uint16) bool {
	if vendorID != 0x1908 {
		return false
	}
	return productID == 0x0102 || productID == 0x0103
}

// VendorIDs returns all vendor IDs this profile supports.
func (p *QTKeJiProfile) VendorIDs() []uint16 {
	return []uint16{0x1908}
}

// ProductIDs returns all product IDs this profile supports.
func (p *QTKeJiProfile) ProductIDs() []uint16 {
	return []uint16{0x0102, 0x0103}
}

// Width returns the display width in pixels.
func (p *QTKeJiProfile) Width() int {
	return qtkejiWidth
}

// Height returns the display height in pixels.
func (p *QTKeJiProfile) Height() int {
	return qtkejiHeight
}

// ColorFormat returns the pixel color format.
func (p *QTKeJiProfile) ColorFormat() ColorFormat {
	return RGB565
}

// ByteOrder returns the byte order for pixel data.
func (p *QTKeJiProfile) ByteOrder() ByteOrder {
	return BigEndian
}

// BufferSize returns the total frame buffer size in bytes.
func (p *QTKeJiProfile) BufferSize() int {
	return qtkejiWidth * qtkejiHeight * p.ColorFormat().BytesPerPixel()
}

// MaxBrightness returns the maximum brightness level.
func (p *QTKeJiProfile) MaxBrightness() int {
	return qtkejiMaxBrightness
}

// ProtocolType returns the USB protocol type.
func (p *QTKeJiProfile) ProtocolType() ProtocolType {
	return ProtocolSCSI
}

// BlitCommand builds the SCSI CBW for sending image data.
//
// QTKeJi/AIDA64 Protocol:
//   - cmd[0] = 0xCD (vendor prefix)
//   - cmd[5] = 0x06 (BLIT operation type)
//   - cmd[6] = 0x12 (write image subcommand)
//   - cmd[7-8] = x0 (little-endian)
//   - cmd[9-10] = y0 (little-endian)
//   - cmd[11-12] = x1 (little-endian)
//   - cmd[13-14] = y1 (little-endian)
func (p *QTKeJiProfile) BlitCommand(x, y, w, h int, dataLen int) []byte {
	cmd := make([]byte, 16)
	cmd[0] = vendorCmdPrefix // 0xCD
	cmd[5] = subcmdBlit      // 0x06
	cmd[6] = blitCmdWrite    // 0x12
	binary.LittleEndian.PutUint16(cmd[7:9], uint16(x))
	binary.LittleEndian.PutUint16(cmd[9:11], uint16(y))
	binary.LittleEndian.PutUint16(cmd[11:13], uint16(x+w-1)) // x1 = x + width - 1
	binary.LittleEndian.PutUint16(cmd[13:15], uint16(y+h-1)) // y1 = y + height - 1

	return p.buildCBW(cmd, uint32(dataLen), dirOut)
}

// BacklightCommand builds the SCSI CBW for setting backlight brightness.
//
// QTKeJi/AIDA64 Protocol:
//   - cmd[0] = 0xCD (vendor prefix)
//   - cmd[5] = 0x06 (BLIT/display operation)
//   - cmd[6] = 0x01 (set property subcommand)
//   - cmd[7] = 0x01 (property enabled)
//   - cmd[8] = 0x00
//   - cmd[9] = brightness level (0-7)
//   - cmd[10] = 0x00
func (p *QTKeJiProfile) BacklightCommand(level int) []byte {
	// Clamp level to valid range
	if level < 0 {
		level = 0
	}
	if level > qtkejiMaxBrightness {
		level = qtkejiMaxBrightness
	}

	cmd := make([]byte, 16)
	cmd[0] = vendorCmdPrefix // 0xCD
	cmd[5] = subcmdBlit      // 0x06
	cmd[6] = blitCmdSetProp  // 0x01
	cmd[7] = 0x01            // Enable
	cmd[8] = 0x00
	cmd[9] = byte(level)
	cmd[10] = 0x00

	return p.buildCBW(cmd, 0, dirOut)
}

// ParseResponse validates a CSW response from the device.
func (p *QTKeJiProfile) ParseResponse(data []byte) error {
	if len(data) < cswLength {
		return fmt.Errorf("CSW response too short: got %d bytes, expected %d", len(data), cswLength)
	}

	sig := string(data[0:4])
	if sig != cswSignature {
		return fmt.Errorf("invalid CSW signature: got %q, expected %q", sig, cswSignature)
	}

	status := data[12]
	if status != 0 {
		return fmt.Errorf("CSW status error: %d", status)
	}

	return nil
}

// ConvertImage converts a Go image to the device's native pixel format.
func (p *QTKeJiProfile) ConvertImage(img image.Image) []byte {
	bounds := img.Bounds()
	buffer := make([]byte, p.BufferSize())

	idx := 0
	for y := 0; y < qtkejiHeight; y++ {
		srcY := bounds.Min.Y + (y * bounds.Dy() / qtkejiHeight)
		for x := 0; x < qtkejiWidth; x++ {
			srcX := bounds.Min.X + (x * bounds.Dx() / qtkejiWidth)

			c := img.At(srcX, srcY)
			r, g, b, _ := c.RGBA()

			// RGBA returns 16-bit values, convert to 8-bit, then to RGB565
			rgb565 := rgbToRGB565(uint8(r>>8), uint8(g>>8), uint8(b>>8))

			// QTKeJi uses big-endian byte order
			binary.BigEndian.PutUint16(buffer[idx:idx+2], rgb565)
			idx += 2
		}
	}

	return buffer
}

// buildCBW creates a SCSI Command Block Wrapper.
func (p *QTKeJiProfile) buildCBW(command []byte, dataLength uint32, direction byte) []byte {
	cbw := make([]byte, cbwLength)

	// Signature
	copy(cbw[0:4], cbwSignature)

	// Tag (little-endian)
	binary.LittleEndian.PutUint32(cbw[4:8], defaultTag)

	// Data transfer length (little-endian)
	binary.LittleEndian.PutUint32(cbw[8:12], dataLength)

	// Direction flag
	cbw[12] = direction

	// LUN (always 0)
	cbw[13] = 0

	// Command block length
	cbw[14] = byte(len(command))

	// Command block (padded to 16 bytes)
	copy(cbw[15:31], command)

	return cbw
}

// rgbToRGB565 converts 24-bit RGB to 16-bit RGB565.
func rgbToRGB565(r, g, b uint8) uint16 {
	r5 := uint16(r>>3) & 0x1F
	g6 := uint16(g>>2) & 0x3F
	b5 := uint16(b>>3) & 0x1F
	return (r5 << 11) | (g6 << 5) | b5
}

// CreateSolidColorBuffer creates an RGB565 buffer filled with a single color.
func (p *QTKeJiProfile) CreateSolidColorBuffer(r, g, b uint8) []byte {
	rgb565 := rgbToRGB565(r, g, b)
	buffer := make([]byte, p.BufferSize())

	for i := 0; i < len(buffer); i += 2 {
		binary.BigEndian.PutUint16(buffer[i:i+2], rgb565)
	}
	return buffer
}

// CreateTestPatternBuffer creates a 4-color quadrant test pattern.
func (p *QTKeJiProfile) CreateTestPatternBuffer() []byte {
	buffer := make([]byte, p.BufferSize())

	red := rgbToRGB565(255, 0, 0)
	green := rgbToRGB565(0, 255, 0)
	blue := rgbToRGB565(0, 0, 255)
	white := rgbToRGB565(255, 255, 255)

	halfW := qtkejiWidth / 2
	halfH := qtkejiHeight / 2

	idx := 0
	for y := 0; y < qtkejiHeight; y++ {
		for x := 0; x < qtkejiWidth; x++ {
			var rgb565 uint16
			if y < halfH {
				if x < halfW {
					rgb565 = red
				} else {
					rgb565 = green
				}
			} else {
				if x < halfW {
					rgb565 = blue
				} else {
					rgb565 = white
				}
			}
			binary.BigEndian.PutUint16(buffer[idx:idx+2], rgb565)
			idx += 2
		}
	}

	return buffer
}
