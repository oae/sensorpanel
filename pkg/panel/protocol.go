// Package panel implements the USB protocol for AX206-based displays.
//
// This module implements the USB protocol for AX206-based digital photo frames
// that use SCSI-like bulk transfer protocol.
//
// Protocol overview:
//   - Uses USB bulk transfers
//   - Commands are wrapped in SCSI Command Block Wrapper (CBW) format
//   - Vendor-specific commands use 0xCD prefix
//   - After data transfer, device sends CSW (Command Status Wrapper) response
//   - Pixel data is sent as raw RGB565 (16-bit, BIG-ENDIAN byte order!)
//
// Device selection:
//   - No hardcoded VID/PID - device must be configured via 'sensorpanel device select'
//   - USB endpoints are detected dynamically from device descriptors
package panel

import (
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"

	"github.com/oae/sensorpanel/pkg/device"
)

// Display dimensions (detected devices may vary - these are common defaults)
const (
	DefaultDisplayWidth  = 480
	DefaultDisplayHeight = 320
	BytesPerPixel        = 2                                                          // RGB565
	DefaultBufferSize    = DefaultDisplayWidth * DefaultDisplayHeight * BytesPerPixel // 307,200 bytes
)

// DeviceInfo contains information about a connected AX206 display.
type DeviceInfo struct {
	// USB descriptor info
	VendorID     uint16
	ProductID    uint16
	Manufacturer string
	Product      string
	Serial       string

	// Display properties
	Width      int
	Height     int
	BufferSize int

	// USB properties
	Speed         string // "low", "full", "high", "super"
	MaxPacketSize int
}

// NewDeviceInfo creates a DeviceInfo with default display dimensions.
// VendorID and ProductID must be set from config or device detection.
func NewDeviceInfo() *DeviceInfo {
	return &DeviceInfo{
		Width:         DefaultDisplayWidth,
		Height:        DefaultDisplayHeight,
		BufferSize:    DefaultBufferSize,
		MaxPacketSize: 64,
	}
}

// NewDeviceInfoFromProfile creates a DeviceInfo from a device profile.
func NewDeviceInfoFromProfile(profile device.DeviceProfile) *DeviceInfo {
	return &DeviceInfo{
		Width:         profile.Width(),
		Height:        profile.Height(),
		BufferSize:    profile.BufferSize(),
		MaxPacketSize: 64,
	}
}

// TheoreticalFPS returns the maximum theoretical FPS based on USB speed.
func (d *DeviceInfo) TheoreticalFPS() float64 {
	// USB speed throughput estimates (usable bandwidth)
	var throughputKBs float64
	switch d.Speed {
	case "low":
		throughputKBs = 1.5 // 1.5 Mbps / 8 * efficiency
	case "full":
		throughputKBs = 500 // 12 Mbps, ~500 KB/s usable
	case "high":
		throughputKBs = 40000 // 480 Mbps, ~40 MB/s usable
	case "super":
		throughputKBs = 400000 // 5 Gbps, ~400 MB/s usable
	default:
		throughputKBs = 500 // Assume full speed
	}
	return throughputKBs * 1024 / float64(d.BufferSize)
}

// String returns a human-readable summary of the device info.
func (d *DeviceInfo) String() string {
	return fmt.Sprintf("%s %s (%04X:%04X) %dx%d @ %s speed",
		d.Manufacturer, d.Product, d.VendorID, d.ProductID,
		d.Width, d.Height, d.Speed)
}

// USB timeouts in milliseconds
const (
	USBTimeout     = 5000  // For commands
	USBTimeoutData = 10000 // For data transfers
)

// SCSI CBW/CSW constants
const (
	CBWSignature = "USBC"
	CSWSignature = "USBS"
	CBWLength    = 31
	CSWLength    = 13
)

// Direction flags
const (
	DirOut = 0x00 // Host to device
	DirIn  = 0x80 // Device to host
)

// Vendor command constants
const (
	VendorCmdPrefix = 0xCD

	// cmd[5] = operation type
	SubcmdGetParam = 0x02 // Get LCD parameters (not supported by QTKeJi)
	SubcmdProbe    = 0x03 // Probe protocol version (not supported by QTKeJi)
	SubcmdBlit     = 0x06 // Blit operation

	// cmd[6] = subcommand (used with SubcmdBlit)
	BlitCmdSetProp = 0x01 // Set property (brightness, etc)
	BlitCmdWrite   = 0x12 // Write image data to screen
)

// Backlight levels
const (
	BacklightOff = 0
	BacklightMax = 7
)

// Errors
var (
	ErrCSWTooShort        = errors.New("CSW response too short")
	ErrCSWInvalidSig      = errors.New("invalid CSW signature")
	ErrCommandTooLong     = errors.New("command block cannot exceed 16 bytes")
	ErrBufferSizeMismatch = errors.New("buffer size mismatch")
)

// BuildCBW creates a SCSI Command Block Wrapper.
//
// CBW Structure (31 bytes):
//   - Bytes 0-3: Signature ('USBC')
//   - Bytes 4-7: Tag (echoed in CSW response)
//   - Bytes 8-11: Data Transfer Length (little-endian)
//   - Byte 12: Flags (0x80 = data in, 0x00 = data out)
//   - Byte 13: LUN (logical unit number)
//   - Byte 14: CB Length (command block length, 1-16)
//   - Bytes 15-30: Command Block (16 bytes, padded with zeros)
func BuildCBW(command []byte, dataLength uint32, direction byte, tag uint32) ([]byte, error) {
	if len(command) > 16 {
		return nil, ErrCommandTooLong
	}

	cbw := make([]byte, CBWLength)

	// Signature
	copy(cbw[0:4], CBWSignature)

	// Tag (little-endian)
	binary.LittleEndian.PutUint32(cbw[4:8], tag)

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

	return cbw, nil
}

// ParseCSW parses a Command Status Wrapper response.
//
// CSW Structure (13 bytes):
//   - Bytes 0-3: Signature ('USBS')
//   - Bytes 4-7: Tag (matches CBW tag)
//   - Bytes 8-11: Data Residue
//   - Byte 12: Status (0 = success)
//
// Returns (tag, status, error)
func ParseCSW(csw []byte) (uint32, byte, error) {
	if len(csw) < CSWLength {
		return 0, 0, fmt.Errorf("%w: got %d bytes, expected %d", ErrCSWTooShort, len(csw), CSWLength)
	}

	sig := string(csw[0:4])
	if sig != CSWSignature {
		return 0, 0, fmt.Errorf("%w: got %q, expected %q", ErrCSWInvalidSig, sig, CSWSignature)
	}

	tag := binary.LittleEndian.Uint32(csw[4:8])
	status := csw[12]

	return tag, status, nil
}

// BuildBlitCmd creates a CBW for blitting image data to the display.
//
// QTKeJi/AIDA64 Protocol:
//   - cmd[0] = 0xCD (vendor prefix)
//   - cmd[5] = 0x06 (BLIT operation type)
//   - cmd[6] = 0x12 (write image subcommand)
//   - cmd[7-8] = x0 (little-endian)
//   - cmd[9-10] = y0 (little-endian)
//   - cmd[11-12] = x1 (little-endian)
//   - cmd[13-14] = y1 (little-endian)
func BuildBlitCmd(x0, y0, x1, y1 int) ([]byte, error) {
	width := x1 - x0 + 1
	height := y1 - y0 + 1
	dataLength := uint32(width * height * BytesPerPixel)

	cmd := make([]byte, 16)
	cmd[0] = VendorCmdPrefix // 0xCD
	cmd[5] = SubcmdBlit      // 0x06
	cmd[6] = BlitCmdWrite    // 0x12
	binary.LittleEndian.PutUint16(cmd[7:9], uint16(x0))
	binary.LittleEndian.PutUint16(cmd[9:11], uint16(y0))
	binary.LittleEndian.PutUint16(cmd[11:13], uint16(x1))
	binary.LittleEndian.PutUint16(cmd[13:15], uint16(y1))

	return BuildCBW(cmd, dataLength, DirOut, 0xDEADBEEF)
}

// BuildFullScreenBlitCmd creates a CBW for full-screen blit (480x320).
func BuildFullScreenBlitCmd() ([]byte, error) {
	return BuildBlitCmd(0, 0, DefaultDisplayWidth-1, DefaultDisplayHeight-1)
}

// BuildSetBacklightCmd creates a CBW for setting backlight brightness.
//
// QTKeJi/AIDA64 Protocol:
//   - cmd[0] = 0xCD (vendor prefix)
//   - cmd[5] = 0x06 (BLIT/display operation)
//   - cmd[6] = 0x01 (set property subcommand)
//   - cmd[7] = 0x01 (property enabled)
//   - cmd[8] = 0x00
//   - cmd[9] = brightness level (0-7)
//   - cmd[10] = 0x00
//
// level: 0 (off) to 7 (brightest)
func BuildSetBacklightCmd(level int) ([]byte, error) {
	// Clamp level to valid range
	if level < 0 {
		level = 0
	}
	if level > 7 {
		level = 7
	}

	cmd := make([]byte, 16)
	cmd[0] = VendorCmdPrefix // 0xCD
	cmd[5] = SubcmdBlit      // 0x06
	cmd[6] = BlitCmdSetProp  // 0x01
	cmd[7] = 0x01            // Enable
	cmd[8] = 0x00
	cmd[9] = byte(level)
	cmd[10] = 0x00

	return BuildCBW(cmd, 0, DirOut, 0xDEADBEEF)
}

// RGBToRGB565 converts 24-bit RGB to 16-bit RGB565.
//
// Standard RGB565 format:
//   - Red: 5 bits (bits 15-11)
//   - Green: 6 bits (bits 10-5)
//   - Blue: 5 bits (bits 4-0)
func RGBToRGB565(r, g, b uint8) uint16 {
	r5 := uint16(r>>3) & 0x1F
	g6 := uint16(g>>2) & 0x3F
	b5 := uint16(b>>3) & 0x1F
	return (r5 << 11) | (g6 << 5) | b5
}

// RGB565ToBytes converts RGB565 value to bytes for wire transfer.
//
// IMPORTANT: QTKeJi/AIDA64 devices expect BIG-ENDIAN byte order!
// (high byte first, low byte second)
func RGB565ToBytes(rgb565 uint16) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, rgb565) // BIG-ENDIAN!
	return buf
}

// CreateSolidColorBuffer creates an RGB565 buffer filled with a single color.
func CreateSolidColorBuffer(r, g, b uint8) []byte {
	rgb565 := RGBToRGB565(r, g, b)
	pixelBytes := RGB565ToBytes(rgb565)

	buffer := make([]byte, DefaultBufferSize)
	for i := 0; i < DefaultBufferSize; i += 2 {
		buffer[i] = pixelBytes[0]
		buffer[i+1] = pixelBytes[1]
	}
	return buffer
}

// CreateTestPatternBuffer creates a 4-color quadrant test pattern.
// Top-left: Red, Top-right: Green, Bottom-left: Blue, Bottom-right: White
func CreateTestPatternBuffer() []byte {
	buffer := make([]byte, DefaultBufferSize)

	red := RGBToRGB565(255, 0, 0)
	green := RGBToRGB565(0, 255, 0)
	blue := RGBToRGB565(0, 0, 255)
	white := RGBToRGB565(255, 255, 255)

	halfW := DefaultDisplayWidth / 2
	halfH := DefaultDisplayHeight / 2

	idx := 0
	for y := 0; y < DefaultDisplayHeight; y++ {
		for x := 0; x < DefaultDisplayWidth; x++ {
			var rgb565 uint16
			if y < halfH {
				if x < halfW {
					rgb565 = red // Top-left
				} else {
					rgb565 = green // Top-right
				}
			} else {
				if x < halfW {
					rgb565 = blue // Bottom-left
				} else {
					rgb565 = white // Bottom-right
				}
			}
			binary.BigEndian.PutUint16(buffer[idx:idx+2], rgb565)
			idx += 2
		}
	}

	return buffer
}

// ImageToRGB565Buffer converts a Go image to RGB565 buffer.
// The image will be resized to DefaultDisplayWidth x DefaultDisplayHeight if needed.
func ImageToRGB565Buffer(img image.Image) []byte {
	bounds := img.Bounds()
	buffer := make([]byte, DefaultBufferSize)

	idx := 0
	for y := 0; y < DefaultDisplayHeight; y++ {
		srcY := bounds.Min.Y + (y * bounds.Dy() / DefaultDisplayHeight)
		for x := 0; x < DefaultDisplayWidth; x++ {
			srcX := bounds.Min.X + (x * bounds.Dx() / DefaultDisplayWidth)

			c := img.At(srcX, srcY)
			r, g, b, _ := c.RGBA()

			// RGBA returns 16-bit values, convert to 8-bit
			rgb565 := RGBToRGB565(uint8(r>>8), uint8(g>>8), uint8(b>>8))
			binary.BigEndian.PutUint16(buffer[idx:idx+2], rgb565)
			idx += 2
		}
	}

	return buffer
}

// ImageToRGB565BufferBE converts a Go image to RGB565 buffer with big-endian byte order.
func ImageToRGB565BufferBE(img image.Image) []byte {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	buffer := make([]byte, width*height*BytesPerPixel)

	idx := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			rgb565 := RGBToRGB565(uint8(r>>8), uint8(g>>8), uint8(b>>8))
			binary.BigEndian.PutUint16(buffer[idx:idx+2], rgb565)
			idx += 2
		}
	}

	return buffer
}

// ImageToRGB565BufferLE converts a Go image to RGB565 buffer with little-endian byte order.
func ImageToRGB565BufferLE(img image.Image) []byte {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	buffer := make([]byte, width*height*BytesPerPixel)

	idx := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			rgb565 := RGBToRGB565(uint8(r>>8), uint8(g>>8), uint8(b>>8))
			binary.LittleEndian.PutUint16(buffer[idx:idx+2], rgb565)
			idx += 2
		}
	}

	return buffer
}

// ImageToRGB888Buffer converts a Go image to RGB888 buffer.
func ImageToRGB888Buffer(img image.Image) []byte {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	buffer := make([]byte, width*height*3)

	idx := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := img.At(x, y)
			r, g, b, _ := c.RGBA()
			buffer[idx] = uint8(r >> 8)
			buffer[idx+1] = uint8(g >> 8)
			buffer[idx+2] = uint8(b >> 8)
			idx += 3
		}
	}

	return buffer
}

// CreateColorBarsBuffer creates a standard 8-color bar test pattern.
func CreateColorBarsBuffer() []byte {
	return CreateColorBarsBufferWithSize(DefaultDisplayWidth, DefaultDisplayHeight)
}

// CreateSolidColorBufferWithSize creates an RGB565 buffer filled with a single color.
func CreateSolidColorBufferWithSize(r, g, b uint8, width, height int) []byte {
	rgb565 := RGBToRGB565(r, g, b)
	bufferSize := width * height * BytesPerPixel
	buffer := make([]byte, bufferSize)

	for i := 0; i < bufferSize; i += 2 {
		binary.BigEndian.PutUint16(buffer[i:i+2], rgb565)
	}
	return buffer
}

// CreateTestPatternBufferWithSize creates a 4-color quadrant test pattern.
func CreateTestPatternBufferWithSize(width, height int) []byte {
	bufferSize := width * height * BytesPerPixel
	buffer := make([]byte, bufferSize)

	red := RGBToRGB565(255, 0, 0)
	green := RGBToRGB565(0, 255, 0)
	blue := RGBToRGB565(0, 0, 255)
	white := RGBToRGB565(255, 255, 255)

	halfW := width / 2
	halfH := height / 2

	idx := 0
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
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

// CreateColorBarsBufferWithSize creates a standard 8-color bar test pattern.
func CreateColorBarsBufferWithSize(width, height int) []byte {
	bufferSize := width * height * BytesPerPixel
	buffer := make([]byte, bufferSize)

	colors := []color.RGBA{
		{255, 255, 255, 255}, // White
		{255, 255, 0, 255},   // Yellow
		{0, 255, 255, 255},   // Cyan
		{0, 255, 0, 255},     // Green
		{255, 0, 255, 255},   // Magenta
		{255, 0, 0, 255},     // Red
		{0, 0, 255, 255},     // Blue
		{0, 0, 0, 255},       // Black
	}

	barHeight := height / len(colors)

	idx := 0
	for y := 0; y < height; y++ {
		colorIdx := y / barHeight
		if colorIdx >= len(colors) {
			colorIdx = len(colors) - 1
		}
		c := colors[colorIdx]
		rgb565 := RGBToRGB565(c.R, c.G, c.B)

		for x := 0; x < width; x++ {
			binary.BigEndian.PutUint16(buffer[idx:idx+2], rgb565)
			idx += 2
		}
	}

	return buffer
}
