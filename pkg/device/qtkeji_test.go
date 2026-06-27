package device

import (
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

func TestQTKeJiProfile_Identity(t *testing.T) {
	p := &QTKeJiProfile{}

	if p.ID() != "qtkeji" {
		t.Errorf("ID() = %q, want %q", p.ID(), "qtkeji")
	}

	if p.Name() != "QTKeJi USB Display" {
		t.Errorf("Name() = %q, want %q", p.Name(), "QTKeJi USB Display")
	}

	if p.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestQTKeJiProfile_Matches(t *testing.T) {
	p := &QTKeJiProfile{}

	tests := []struct {
		vid, pid uint16
		want     bool
	}{
		{0x1908, 0x0102, true},
		{0x1908, 0x0103, true},
		{0x1908, 0x0104, false}, // Unknown PID
		{0x1909, 0x0102, false}, // Wrong VID
		{0x0000, 0x0000, false},
	}

	for _, tt := range tests {
		got := p.Matches(tt.vid, tt.pid)
		if got != tt.want {
			t.Errorf("Matches(%04x, %04x) = %v, want %v", tt.vid, tt.pid, got, tt.want)
		}
	}
}

func TestQTKeJiProfile_VIDPIDs(t *testing.T) {
	p := &QTKeJiProfile{}

	vids := p.VendorIDs()
	if len(vids) != 1 || vids[0] != 0x1908 {
		t.Errorf("VendorIDs() = %v, want [0x1908]", vids)
	}

	pids := p.ProductIDs()
	if len(pids) != 2 {
		t.Errorf("ProductIDs() length = %d, want 2", len(pids))
	}
	if pids[0] != 0x0102 || pids[1] != 0x0103 {
		t.Errorf("ProductIDs() = %v, want [0x0102, 0x0103]", pids)
	}
}

func TestQTKeJiProfile_DisplayProperties(t *testing.T) {
	p := &QTKeJiProfile{}

	if p.Width() != 480 {
		t.Errorf("Width() = %d, want 480", p.Width())
	}

	if p.Height() != 320 {
		t.Errorf("Height() = %d, want 320", p.Height())
	}

	if p.ColorFormat() != RGB565 {
		t.Errorf("ColorFormat() = %v, want RGB565", p.ColorFormat())
	}

	if p.ByteOrder() != BigEndian {
		t.Errorf("ByteOrder() = %v, want BigEndian", p.ByteOrder())
	}

	expectedSize := 480 * 320 * 2
	if p.BufferSize() != expectedSize {
		t.Errorf("BufferSize() = %d, want %d", p.BufferSize(), expectedSize)
	}

	if p.MaxBrightness() != 7 {
		t.Errorf("MaxBrightness() = %d, want 7", p.MaxBrightness())
	}

	if p.ProtocolType() != ProtocolSCSI {
		t.Errorf("ProtocolType() = %v, want ProtocolSCSI", p.ProtocolType())
	}
}

func TestQTKeJiProfile_BlitCommand(t *testing.T) {
	p := &QTKeJiProfile{}

	cmd := p.BlitCommand(12, 34, 56, 78, 56*78*2)

	// CBW should be 31 bytes
	if len(cmd) != 31 {
		t.Errorf("BlitCommand() length = %d, want 31", len(cmd))
	}

	// Check CBW signature
	if string(cmd[0:4]) != "USBC" {
		t.Errorf("CBW signature = %q, want %q", string(cmd[0:4]), "USBC")
	}

	// Check command block starts at offset 15
	if cmd[15] != 0xCD {
		t.Errorf("cmd[15] = %02x, want 0xCD (vendor prefix)", cmd[15])
	}

	if cmd[20] != 0x06 {
		t.Errorf("cmd[20] = %02x, want 0x06 (blit)", cmd[20])
	}

	if cmd[21] != 0x12 {
		t.Errorf("cmd[21] = %02x, want 0x12 (write)", cmd[21])
	}

	if got := binary.LittleEndian.Uint16(cmd[22:24]); got != 12 {
		t.Errorf("x0 = %d, want 12", got)
	}
	if got := binary.LittleEndian.Uint16(cmd[24:26]); got != 34 {
		t.Errorf("y0 = %d, want 34", got)
	}
	if got := binary.LittleEndian.Uint16(cmd[26:28]); got != 67 {
		t.Errorf("x1 = %d, want 67", got)
	}
	if got := binary.LittleEndian.Uint16(cmd[28:30]); got != 111 {
		t.Errorf("y1 = %d, want 111", got)
	}
}

func TestQTKeJiProfile_BacklightCommand(t *testing.T) {
	p := &QTKeJiProfile{}

	tests := []struct {
		level     int
		wantLevel int // clamped
	}{
		{0, 0},
		{7, 7},
		{3, 3},
		{-1, 0},  // clamped to min
		{10, 7},  // clamped to max
		{100, 7}, // clamped to max
	}

	for _, tt := range tests {
		cmd := p.BacklightCommand(tt.level)

		// CBW should be 31 bytes
		if len(cmd) != 31 {
			t.Errorf("BacklightCommand(%d) length = %d, want 31", tt.level, len(cmd))
		}

		// Check CBW signature
		if string(cmd[0:4]) != "USBC" {
			t.Errorf("BacklightCommand(%d) CBW signature = %q, want %q", tt.level, string(cmd[0:4]), "USBC")
		}

		// Check brightness level in command block (offset 15 + 9 = 24)
		if int(cmd[24]) != tt.wantLevel {
			t.Errorf("BacklightCommand(%d) brightness byte = %d, want %d", tt.level, cmd[24], tt.wantLevel)
		}
	}
}

func TestQTKeJiProfile_ParseResponse(t *testing.T) {
	p := &QTKeJiProfile{}

	// Valid CSW response
	validCSW := make([]byte, 13)
	copy(validCSW[0:4], "USBS")
	validCSW[12] = 0 // Success status

	if err := p.ParseResponse(validCSW); err != nil {
		t.Errorf("ParseResponse(valid CSW) = %v, want nil", err)
	}

	// CSW too short
	shortCSW := make([]byte, 5)
	if err := p.ParseResponse(shortCSW); err == nil {
		t.Error("ParseResponse(short CSW) = nil, want error")
	}

	// Invalid signature
	invalidSig := make([]byte, 13)
	copy(invalidSig[0:4], "XXXX")
	if err := p.ParseResponse(invalidSig); err == nil {
		t.Error("ParseResponse(invalid signature) = nil, want error")
	}

	// Non-zero status
	errorCSW := make([]byte, 13)
	copy(errorCSW[0:4], "USBS")
	errorCSW[12] = 1 // Error status
	if err := p.ParseResponse(errorCSW); err == nil {
		t.Error("ParseResponse(error status) = nil, want error")
	}
}

func TestQTKeJiProfile_ConvertImage(t *testing.T) {
	p := &QTKeJiProfile{}

	// Create a simple test image
	img := image.NewRGBA(image.Rect(0, 0, 480, 320))
	for y := 0; y < 320; y++ {
		for x := 0; x < 480; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255}) // Red
		}
	}

	buffer := p.ConvertImage(img)

	expectedSize := 480 * 320 * 2
	if len(buffer) != expectedSize {
		t.Errorf("ConvertImage() buffer size = %d, want %d", len(buffer), expectedSize)
	}

	// Check first pixel is red in RGB565 big-endian
	// Red in RGB565: R=31, G=0, B=0 = 0xF800
	// Big-endian: 0xF8, 0x00
	if buffer[0] != 0xF8 || buffer[1] != 0x00 {
		t.Errorf("ConvertImage() first pixel = %02x%02x, want F800 (red)", buffer[0], buffer[1])
	}
}

func TestQTKeJiProfile_CreateSolidColorBuffer(t *testing.T) {
	p := &QTKeJiProfile{}

	buffer := p.CreateSolidColorBuffer(0, 255, 0) // Green

	expectedSize := 480 * 320 * 2
	if len(buffer) != expectedSize {
		t.Errorf("CreateSolidColorBuffer() size = %d, want %d", len(buffer), expectedSize)
	}

	// Green in RGB565: R=0, G=63, B=0 = 0x07E0
	// Big-endian: 0x07, 0xE0
	if buffer[0] != 0x07 || buffer[1] != 0xE0 {
		t.Errorf("CreateSolidColorBuffer(green) first pixel = %02x%02x, want 07E0", buffer[0], buffer[1])
	}

	// Check all pixels are the same
	for i := 0; i < len(buffer); i += 2 {
		if buffer[i] != 0x07 || buffer[i+1] != 0xE0 {
			t.Errorf("CreateSolidColorBuffer() pixel at %d = %02x%02x, want 07E0", i/2, buffer[i], buffer[i+1])
			break
		}
	}
}

func TestQTKeJiProfile_CreateTestPatternBuffer(t *testing.T) {
	p := &QTKeJiProfile{}

	buffer := p.CreateTestPatternBuffer()

	expectedSize := 480 * 320 * 2
	if len(buffer) != expectedSize {
		t.Errorf("CreateTestPatternBuffer() size = %d, want %d", len(buffer), expectedSize)
	}

	// Check top-left is red (0xF800)
	if buffer[0] != 0xF8 || buffer[1] != 0x00 {
		t.Errorf("CreateTestPatternBuffer() top-left = %02x%02x, want F800 (red)", buffer[0], buffer[1])
	}

	// Check top-right is green (0x07E0) - first pixel past middle
	topRightIdx := (240) * 2 // x=240, y=0
	if buffer[topRightIdx] != 0x07 || buffer[topRightIdx+1] != 0xE0 {
		t.Errorf("CreateTestPatternBuffer() top-right = %02x%02x, want 07E0 (green)", buffer[topRightIdx], buffer[topRightIdx+1])
	}

	// Check bottom-left is blue (0x001F) - first pixel of second half
	bottomLeftIdx := (160 * 480) * 2 // x=0, y=160
	if buffer[bottomLeftIdx] != 0x00 || buffer[bottomLeftIdx+1] != 0x1F {
		t.Errorf("CreateTestPatternBuffer() bottom-left = %02x%02x, want 001F (blue)", buffer[bottomLeftIdx], buffer[bottomLeftIdx+1])
	}

	// Check bottom-right is white (0xFFFF)
	bottomRightIdx := (160*480 + 240) * 2 // x=240, y=160
	if buffer[bottomRightIdx] != 0xFF || buffer[bottomRightIdx+1] != 0xFF {
		t.Errorf("CreateTestPatternBuffer() bottom-right = %02x%02x, want FFFF (white)", buffer[bottomRightIdx], buffer[bottomRightIdx+1])
	}
}

func TestRgbToRGB565(t *testing.T) {
	tests := []struct {
		r, g, b uint8
		want    uint16
	}{
		{255, 0, 0, 0xF800},     // Red
		{0, 255, 0, 0x07E0},     // Green
		{0, 0, 255, 0x001F},     // Blue
		{255, 255, 255, 0xFFFF}, // White
		{0, 0, 0, 0x0000},       // Black
		{128, 128, 128, 0x8410}, // Gray (approximately)
	}

	for _, tt := range tests {
		got := rgbToRGB565(tt.r, tt.g, tt.b)
		if got != tt.want {
			t.Errorf("rgbToRGB565(%d, %d, %d) = %04X, want %04X", tt.r, tt.g, tt.b, got, tt.want)
		}
	}
}

func TestQTKeJiProfile_buildCBW(t *testing.T) {
	p := &QTKeJiProfile{}

	cmd := make([]byte, 16)
	cmd[0] = 0xCD

	cbw := p.buildCBW(cmd, 1000, 0x00)

	if len(cbw) != 31 {
		t.Errorf("buildCBW() length = %d, want 31", len(cbw))
	}

	// Check signature
	if string(cbw[0:4]) != "USBC" {
		t.Errorf("buildCBW() signature = %q, want %q", string(cbw[0:4]), "USBC")
	}

	// Check tag (little-endian)
	tag := binary.LittleEndian.Uint32(cbw[4:8])
	if tag != 0xDEADBEEF {
		t.Errorf("buildCBW() tag = %08X, want DEADBEEF", tag)
	}

	// Check data length (little-endian)
	dataLen := binary.LittleEndian.Uint32(cbw[8:12])
	if dataLen != 1000 {
		t.Errorf("buildCBW() dataLength = %d, want 1000", dataLen)
	}

	// Check direction
	if cbw[12] != 0x00 {
		t.Errorf("buildCBW() direction = %02X, want 0x00", cbw[12])
	}

	// Check LUN
	if cbw[13] != 0 {
		t.Errorf("buildCBW() LUN = %d, want 0", cbw[13])
	}

	// Check command length
	if cbw[14] != 16 {
		t.Errorf("buildCBW() command length = %d, want 16", cbw[14])
	}

	// Check command block starts with 0xCD
	if cbw[15] != 0xCD {
		t.Errorf("buildCBW() command[0] = %02X, want 0xCD", cbw[15])
	}
}
