package device

import (
	"encoding/binary"
	"fmt"
	"image"
)

// Default values for generic/unknown devices
const (
	genericDefaultWidth         = 480
	genericDefaultHeight        = 320
	genericDefaultMaxBrightness = 7
)

// GenericProfile is a fallback profile for unknown USB display devices.
//
// It uses the same protocol as QTKeJi devices, which is the most common.
// This allows basic functionality even for unrecognized devices.
type GenericProfile struct {
	vendorID  uint16
	productID uint16
	width     int
	height    int
}

// NewGenericProfile creates a generic profile for an unknown device.
func NewGenericProfile(vendorID, productID uint16) *GenericProfile {
	return &GenericProfile{
		vendorID:  vendorID,
		productID: productID,
		width:     genericDefaultWidth,
		height:    genericDefaultHeight,
	}
}

// NewGenericProfileWithSize creates a generic profile with custom dimensions.
func NewGenericProfileWithSize(vendorID, productID uint16, width, height int) *GenericProfile {
	return &GenericProfile{
		vendorID:  vendorID,
		productID: productID,
		width:     width,
		height:    height,
	}
}

// ID returns the unique identifier for this profile.
func (p *GenericProfile) ID() string {
	return "generic"
}

// Name returns the human-readable device name.
func (p *GenericProfile) Name() string {
	return fmt.Sprintf("Unknown Device (%04x:%04x)", p.vendorID, p.productID)
}

// Description returns a brief description.
func (p *GenericProfile) Description() string {
	return "Generic USB display (using QTKeJi-compatible protocol)"
}

// Matches returns true if this profile supports the given VID/PID.
// Generic profile matches only its specific VID/PID, not as a catch-all.
func (p *GenericProfile) Matches(vendorID, productID uint16) bool {
	return p.vendorID == vendorID && p.productID == productID
}

// VendorIDs returns all vendor IDs this profile supports.
func (p *GenericProfile) VendorIDs() []uint16 {
	return []uint16{p.vendorID}
}

// ProductIDs returns all product IDs this profile supports.
func (p *GenericProfile) ProductIDs() []uint16 {
	return []uint16{p.productID}
}

// Width returns the display width in pixels.
func (p *GenericProfile) Width() int {
	return p.width
}

// Height returns the display height in pixels.
func (p *GenericProfile) Height() int {
	return p.height
}

// ColorFormat returns the pixel color format.
func (p *GenericProfile) ColorFormat() ColorFormat {
	return RGB565
}

// ByteOrder returns the byte order for pixel data.
func (p *GenericProfile) ByteOrder() ByteOrder {
	return BigEndian
}

// BufferSize returns the total frame buffer size in bytes.
func (p *GenericProfile) BufferSize() int {
	return p.width * p.height * p.ColorFormat().BytesPerPixel()
}

// MaxBrightness returns the maximum brightness level.
func (p *GenericProfile) MaxBrightness() int {
	return genericDefaultMaxBrightness
}

// ProtocolType returns the USB protocol type.
func (p *GenericProfile) ProtocolType() ProtocolType {
	return ProtocolSCSI
}

// BlitCommand builds the SCSI CBW for sending image data.
// Uses QTKeJi-compatible protocol as a reasonable default.
func (p *GenericProfile) BlitCommand(x, y, w, h int, dataLen int) []byte {
	cmd := make([]byte, 16)
	cmd[0] = vendorCmdPrefix // 0xCD
	cmd[5] = subcmdBlit      // 0x06
	cmd[6] = blitCmdWrite    // 0x12
	binary.LittleEndian.PutUint16(cmd[7:9], uint16(x))
	binary.LittleEndian.PutUint16(cmd[9:11], uint16(y))
	binary.LittleEndian.PutUint16(cmd[11:13], uint16(x+w-1))
	binary.LittleEndian.PutUint16(cmd[13:15], uint16(y+h-1))

	return p.buildCBW(cmd, uint32(dataLen), dirOut)
}

// BacklightCommand builds the SCSI CBW for setting backlight brightness.
func (p *GenericProfile) BacklightCommand(level int) []byte {
	if level < 0 {
		level = 0
	}
	if level > genericDefaultMaxBrightness {
		level = genericDefaultMaxBrightness
	}

	cmd := make([]byte, 16)
	cmd[0] = vendorCmdPrefix
	cmd[5] = subcmdBlit
	cmd[6] = blitCmdSetProp
	cmd[7] = 0x01
	cmd[8] = 0x00
	cmd[9] = byte(level)
	cmd[10] = 0x00

	return p.buildCBW(cmd, 0, dirOut)
}

// ParseResponse validates a CSW response from the device.
func (p *GenericProfile) ParseResponse(data []byte) error {
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
func (p *GenericProfile) ConvertImage(img image.Image) []byte {
	bounds := img.Bounds()
	buffer := make([]byte, p.BufferSize())

	idx := 0
	for y := 0; y < p.height; y++ {
		srcY := bounds.Min.Y + (y * bounds.Dy() / p.height)
		for x := 0; x < p.width; x++ {
			srcX := bounds.Min.X + (x * bounds.Dx() / p.width)

			c := img.At(srcX, srcY)
			r, g, b, _ := c.RGBA()

			rgb565 := rgbToRGB565(uint8(r>>8), uint8(g>>8), uint8(b>>8))
			binary.BigEndian.PutUint16(buffer[idx:idx+2], rgb565)
			idx += 2
		}
	}

	return buffer
}

// buildCBW creates a SCSI Command Block Wrapper.
func (p *GenericProfile) buildCBW(command []byte, dataLength uint32, direction byte) []byte {
	cbw := make([]byte, cbwLength)

	copy(cbw[0:4], cbwSignature)
	binary.LittleEndian.PutUint32(cbw[4:8], defaultTag)
	binary.LittleEndian.PutUint32(cbw[8:12], dataLength)
	cbw[12] = direction
	cbw[13] = 0
	cbw[14] = byte(len(command))
	copy(cbw[15:31], command)

	return cbw
}
