package panel

import (
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/alperen/sensorpanel/pkg/device"
)

func TestNewDeviceInfo(t *testing.T) {
	info := NewDeviceInfo()

	if info.Width != DefaultDisplayWidth {
		t.Errorf("Width = %d, want %d", info.Width, DefaultDisplayWidth)
	}
	if info.Height != DefaultDisplayHeight {
		t.Errorf("Height = %d, want %d", info.Height, DefaultDisplayHeight)
	}
	if info.BufferSize != DefaultBufferSize {
		t.Errorf("BufferSize = %d, want %d", info.BufferSize, DefaultBufferSize)
	}
	if info.MaxPacketSize != 64 {
		t.Errorf("MaxPacketSize = %d, want 64", info.MaxPacketSize)
	}
}

// mockProfile implements device.DeviceProfile for testing
type mockProfile struct{}

func (m *mockProfile) ID() string                                     { return "mock" }
func (m *mockProfile) Name() string                                   { return "Mock Device" }
func (m *mockProfile) Description() string                            { return "Mock device for testing" }
func (m *mockProfile) Matches(vendorID, productID uint16) bool        { return false }
func (m *mockProfile) VendorIDs() []uint16                            { return []uint16{0x1234} }
func (m *mockProfile) ProductIDs() []uint16                           { return []uint16{0x5678} }
func (m *mockProfile) Width() int                                     { return 800 }
func (m *mockProfile) Height() int                                    { return 480 }
func (m *mockProfile) ColorFormat() device.ColorFormat                { return device.RGB565 }
func (m *mockProfile) ByteOrder() device.ByteOrder                    { return device.BigEndian }
func (m *mockProfile) BufferSize() int                                { return 800 * 480 * 2 }
func (m *mockProfile) MaxBrightness() int                             { return 255 }
func (m *mockProfile) ProtocolType() device.ProtocolType              { return device.ProtocolSCSI }
func (m *mockProfile) BlitCommand(x, y, w, h int, dataLen int) []byte { return nil }
func (m *mockProfile) BacklightCommand(level int) []byte              { return nil }
func (m *mockProfile) ParseResponse(data []byte) error                { return nil }
func (m *mockProfile) ConvertImage(img image.Image) []byte            { return nil }

func TestNewDeviceInfoFromProfile(t *testing.T) {
	profile := &mockProfile{}
	info := NewDeviceInfoFromProfile(profile)

	if info.Width != 800 {
		t.Errorf("Width = %d, want 800", info.Width)
	}
	if info.Height != 480 {
		t.Errorf("Height = %d, want 480", info.Height)
	}
	if info.BufferSize != 800*480*2 {
		t.Errorf("BufferSize = %d, want %d", info.BufferSize, 800*480*2)
	}
	if info.MaxPacketSize != 64 {
		t.Errorf("MaxPacketSize = %d, want 64", info.MaxPacketSize)
	}
}

func TestDeviceInfo_TheoreticalFPS(t *testing.T) {
	tests := []struct {
		speed      string
		bufferSize int
		wantMin    float64
		wantMax    float64
	}{
		{"low", DefaultBufferSize, 0.001, 0.01},  // Very low FPS
		{"full", DefaultBufferSize, 1.0, 2.0},    // ~1.6 FPS
		{"high", DefaultBufferSize, 100, 200},    // ~133 FPS
		{"super", DefaultBufferSize, 1000, 2000}, // ~1333 FPS
		{"unknown", DefaultBufferSize, 1.0, 2.0}, // Defaults to full speed
		{"", DefaultBufferSize, 1.0, 2.0},        // Empty defaults to full speed
	}

	for _, tt := range tests {
		t.Run(tt.speed, func(t *testing.T) {
			info := &DeviceInfo{
				Speed:      tt.speed,
				BufferSize: tt.bufferSize,
			}
			fps := info.TheoreticalFPS()
			if fps < tt.wantMin || fps > tt.wantMax {
				t.Errorf("TheoreticalFPS() = %f, want between %f and %f", fps, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestDeviceInfo_String(t *testing.T) {
	info := &DeviceInfo{
		Manufacturer: "TestCo",
		Product:      "TestPanel",
		VendorID:     0x1234,
		ProductID:    0x5678,
		Width:        480,
		Height:       320,
		Speed:        "high",
	}

	str := info.String()

	expected := "TestCo TestPanel (1234:5678) 480x320 @ high speed"
	if str != expected {
		t.Errorf("String() = %q, want %q", str, expected)
	}
}

func TestBuildCBW(t *testing.T) {
	t.Run("valid command", func(t *testing.T) {
		cmd := []byte{0xCD, 0x00, 0x00, 0x00, 0x00, 0x06, 0x12}
		cbw, err := BuildCBW(cmd, 307200, DirOut, 0xDEADBEEF)

		if err != nil {
			t.Fatalf("BuildCBW() error = %v", err)
		}
		if len(cbw) != CBWLength {
			t.Errorf("CBW length = %d, want %d", len(cbw), CBWLength)
		}

		// Check signature
		if string(cbw[0:4]) != CBWSignature {
			t.Errorf("Signature = %q, want %q", string(cbw[0:4]), CBWSignature)
		}

		// Check tag
		tag := binary.LittleEndian.Uint32(cbw[4:8])
		if tag != 0xDEADBEEF {
			t.Errorf("Tag = 0x%X, want 0xDEADBEEF", tag)
		}

		// Check data length
		dataLen := binary.LittleEndian.Uint32(cbw[8:12])
		if dataLen != 307200 {
			t.Errorf("DataLength = %d, want 307200", dataLen)
		}

		// Check direction
		if cbw[12] != DirOut {
			t.Errorf("Direction = 0x%X, want 0x%X", cbw[12], DirOut)
		}

		// Check LUN
		if cbw[13] != 0 {
			t.Errorf("LUN = %d, want 0", cbw[13])
		}

		// Check CB length
		if cbw[14] != byte(len(cmd)) {
			t.Errorf("CB length = %d, want %d", cbw[14], len(cmd))
		}

		// Check command was copied
		if cbw[15] != 0xCD {
			t.Errorf("Command byte 0 = 0x%X, want 0xCD", cbw[15])
		}
	})

	t.Run("direction in", func(t *testing.T) {
		cmd := []byte{0xCD}
		cbw, err := BuildCBW(cmd, 0, DirIn, 0x12345678)

		if err != nil {
			t.Fatalf("BuildCBW() error = %v", err)
		}
		if cbw[12] != DirIn {
			t.Errorf("Direction = 0x%X, want 0x%X", cbw[12], DirIn)
		}
	})

	t.Run("command too long", func(t *testing.T) {
		cmd := make([]byte, 17) // Exceeds 16 byte limit
		_, err := BuildCBW(cmd, 0, DirOut, 0)

		if err != ErrCommandTooLong {
			t.Errorf("BuildCBW() error = %v, want ErrCommandTooLong", err)
		}
	})

	t.Run("max length command", func(t *testing.T) {
		cmd := make([]byte, 16) // Exactly 16 bytes
		cbw, err := BuildCBW(cmd, 0, DirOut, 0)

		if err != nil {
			t.Fatalf("BuildCBW() error = %v", err)
		}
		if cbw[14] != 16 {
			t.Errorf("CB length = %d, want 16", cbw[14])
		}
	})
}

func TestParseCSW(t *testing.T) {
	t.Run("valid CSW", func(t *testing.T) {
		csw := make([]byte, CSWLength)
		copy(csw[0:4], CSWSignature)
		binary.LittleEndian.PutUint32(csw[4:8], 0xDEADBEEF) // Tag
		binary.LittleEndian.PutUint32(csw[8:12], 0)         // Residue
		csw[12] = 0                                         // Status success

		tag, status, err := ParseCSW(csw)

		if err != nil {
			t.Fatalf("ParseCSW() error = %v", err)
		}
		if tag != 0xDEADBEEF {
			t.Errorf("Tag = 0x%X, want 0xDEADBEEF", tag)
		}
		if status != 0 {
			t.Errorf("Status = %d, want 0", status)
		}
	})

	t.Run("error status", func(t *testing.T) {
		csw := make([]byte, CSWLength)
		copy(csw[0:4], CSWSignature)
		binary.LittleEndian.PutUint32(csw[4:8], 0x12345678)
		csw[12] = 1 // Error status

		tag, status, err := ParseCSW(csw)

		if err != nil {
			t.Fatalf("ParseCSW() error = %v", err)
		}
		if tag != 0x12345678 {
			t.Errorf("Tag = 0x%X, want 0x12345678", tag)
		}
		if status != 1 {
			t.Errorf("Status = %d, want 1", status)
		}
	})

	t.Run("CSW too short", func(t *testing.T) {
		csw := make([]byte, CSWLength-1)
		copy(csw[0:4], CSWSignature)

		_, _, err := ParseCSW(csw)

		if err == nil {
			t.Error("ParseCSW() expected error for short CSW, got nil")
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		csw := make([]byte, CSWLength)
		copy(csw[0:4], "XXXX") // Invalid signature

		_, _, err := ParseCSW(csw)

		if err == nil {
			t.Error("ParseCSW() expected error for invalid signature, got nil")
		}
	})
}

func TestBuildBlitCmd(t *testing.T) {
	cbw, err := BuildBlitCmd(0, 0, 479, 319)

	if err != nil {
		t.Fatalf("BuildBlitCmd() error = %v", err)
	}
	if len(cbw) != CBWLength {
		t.Errorf("CBW length = %d, want %d", len(cbw), CBWLength)
	}

	// Check data length (480 * 320 * 2 = 307200)
	dataLen := binary.LittleEndian.Uint32(cbw[8:12])
	if dataLen != DefaultBufferSize {
		t.Errorf("DataLength = %d, want %d", dataLen, DefaultBufferSize)
	}

	// Check vendor command prefix
	if cbw[15] != VendorCmdPrefix {
		t.Errorf("Command[0] = 0x%X, want 0x%X", cbw[15], VendorCmdPrefix)
	}

	// Check operation type
	if cbw[20] != SubcmdBlit {
		t.Errorf("Command[5] = 0x%X, want 0x%X", cbw[20], SubcmdBlit)
	}

	// Check subcommand
	if cbw[21] != BlitCmdWrite {
		t.Errorf("Command[6] = 0x%X, want 0x%X", cbw[21], BlitCmdWrite)
	}
}

func TestBuildFullScreenBlitCmd(t *testing.T) {
	cbw, err := BuildFullScreenBlitCmd()

	if err != nil {
		t.Fatalf("BuildFullScreenBlitCmd() error = %v", err)
	}

	// Should be same as BuildBlitCmd(0, 0, 479, 319)
	expectedCbw, _ := BuildBlitCmd(0, 0, DefaultDisplayWidth-1, DefaultDisplayHeight-1)

	if len(cbw) != len(expectedCbw) {
		t.Errorf("CBW length = %d, want %d", len(cbw), len(expectedCbw))
	}

	for i := range cbw {
		if cbw[i] != expectedCbw[i] {
			t.Errorf("CBW[%d] = 0x%X, want 0x%X", i, cbw[i], expectedCbw[i])
		}
	}
}

func TestBuildSetBacklightCmd(t *testing.T) {
	tests := []struct {
		level    int
		expected int
	}{
		{-5, 0}, // Clamped to 0
		{0, 0},
		{3, 3},
		{7, 7},
		{10, 7},  // Clamped to 7
		{100, 7}, // Clamped to 7
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			cbw, err := BuildSetBacklightCmd(tt.level)

			if err != nil {
				t.Fatalf("BuildSetBacklightCmd(%d) error = %v", tt.level, err)
			}
			if len(cbw) != CBWLength {
				t.Errorf("CBW length = %d, want %d", len(cbw), CBWLength)
			}

			// Check vendor command prefix
			if cbw[15] != VendorCmdPrefix {
				t.Errorf("Command[0] = 0x%X, want 0x%X", cbw[15], VendorCmdPrefix)
			}

			// Check operation type
			if cbw[20] != SubcmdBlit {
				t.Errorf("Command[5] = 0x%X, want 0x%X", cbw[20], SubcmdBlit)
			}

			// Check subcommand
			if cbw[21] != BlitCmdSetProp {
				t.Errorf("Command[6] = 0x%X, want 0x%X", cbw[21], BlitCmdSetProp)
			}

			// Check enable flag
			if cbw[22] != 0x01 {
				t.Errorf("Command[7] = 0x%X, want 0x01", cbw[22])
			}

			// Check brightness level
			if cbw[24] != byte(tt.expected) {
				t.Errorf("Command[9] = %d, want %d", cbw[24], tt.expected)
			}

			// Check data length is 0 (no data transfer)
			dataLen := binary.LittleEndian.Uint32(cbw[8:12])
			if dataLen != 0 {
				t.Errorf("DataLength = %d, want 0", dataLen)
			}
		})
	}
}

func TestRGBToRGB565(t *testing.T) {
	tests := []struct {
		r, g, b  uint8
		expected uint16
	}{
		{0, 0, 0, 0x0000},       // Black
		{255, 255, 255, 0xFFFF}, // White
		{255, 0, 0, 0xF800},     // Red
		{0, 255, 0, 0x07E0},     // Green
		{0, 0, 255, 0x001F},     // Blue
		{255, 255, 0, 0xFFE0},   // Yellow
		{0, 255, 255, 0x07FF},   // Cyan
		{255, 0, 255, 0xF81F},   // Magenta
		{128, 128, 128, 0x8410}, // Mid gray
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := RGBToRGB565(tt.r, tt.g, tt.b)
			if result != tt.expected {
				t.Errorf("RGBToRGB565(%d, %d, %d) = 0x%04X, want 0x%04X", tt.r, tt.g, tt.b, result, tt.expected)
			}
		})
	}
}

func TestRGB565ToBytes(t *testing.T) {
	t.Run("white", func(t *testing.T) {
		bytes := RGB565ToBytes(0xFFFF)
		if bytes[0] != 0xFF || bytes[1] != 0xFF {
			t.Errorf("RGB565ToBytes(0xFFFF) = %v, want [0xFF, 0xFF]", bytes)
		}
	})

	t.Run("red", func(t *testing.T) {
		bytes := RGB565ToBytes(0xF800)
		// Big endian: high byte first
		if bytes[0] != 0xF8 || bytes[1] != 0x00 {
			t.Errorf("RGB565ToBytes(0xF800) = %v, want [0xF8, 0x00]", bytes)
		}
	})

	t.Run("blue", func(t *testing.T) {
		bytes := RGB565ToBytes(0x001F)
		// Big endian: high byte first
		if bytes[0] != 0x00 || bytes[1] != 0x1F {
			t.Errorf("RGB565ToBytes(0x001F) = %v, want [0x00, 0x1F]", bytes)
		}
	})
}

func TestCreateSolidColorBuffer(t *testing.T) {
	t.Run("red buffer", func(t *testing.T) {
		buffer := CreateSolidColorBuffer(255, 0, 0)

		if len(buffer) != DefaultBufferSize {
			t.Errorf("Buffer length = %d, want %d", len(buffer), DefaultBufferSize)
		}

		// Check first pixel is red (0xF800 in big endian)
		if buffer[0] != 0xF8 || buffer[1] != 0x00 {
			t.Errorf("First pixel = [0x%02X, 0x%02X], want [0xF8, 0x00]", buffer[0], buffer[1])
		}

		// Check all pixels are the same
		for i := 0; i < DefaultBufferSize; i += 2 {
			if buffer[i] != 0xF8 || buffer[i+1] != 0x00 {
				t.Errorf("Pixel at %d = [0x%02X, 0x%02X], want [0xF8, 0x00]", i, buffer[i], buffer[i+1])
				break
			}
		}
	})

	t.Run("green buffer", func(t *testing.T) {
		buffer := CreateSolidColorBuffer(0, 255, 0)

		// Check first pixel is green (0x07E0 in big endian)
		if buffer[0] != 0x07 || buffer[1] != 0xE0 {
			t.Errorf("First pixel = [0x%02X, 0x%02X], want [0x07, 0xE0]", buffer[0], buffer[1])
		}
	})
}

func TestCreateTestPatternBuffer(t *testing.T) {
	buffer := CreateTestPatternBuffer()

	if len(buffer) != DefaultBufferSize {
		t.Errorf("Buffer length = %d, want %d", len(buffer), DefaultBufferSize)
	}

	halfW := DefaultDisplayWidth / 2
	halfH := DefaultDisplayHeight / 2

	// Check top-left (red)
	tlIdx := 0
	tlPixel := binary.BigEndian.Uint16(buffer[tlIdx : tlIdx+2])
	if tlPixel != RGBToRGB565(255, 0, 0) {
		t.Errorf("Top-left pixel = 0x%04X, want 0x%04X (red)", tlPixel, RGBToRGB565(255, 0, 0))
	}

	// Check top-right (green)
	trIdx := halfW * 2
	trPixel := binary.BigEndian.Uint16(buffer[trIdx : trIdx+2])
	if trPixel != RGBToRGB565(0, 255, 0) {
		t.Errorf("Top-right pixel = 0x%04X, want 0x%04X (green)", trPixel, RGBToRGB565(0, 255, 0))
	}

	// Check bottom-left (blue)
	blIdx := halfH * DefaultDisplayWidth * 2
	blPixel := binary.BigEndian.Uint16(buffer[blIdx : blIdx+2])
	if blPixel != RGBToRGB565(0, 0, 255) {
		t.Errorf("Bottom-left pixel = 0x%04X, want 0x%04X (blue)", blPixel, RGBToRGB565(0, 0, 255))
	}

	// Check bottom-right (white)
	brIdx := halfH*DefaultDisplayWidth*2 + halfW*2
	brPixel := binary.BigEndian.Uint16(buffer[brIdx : brIdx+2])
	if brPixel != RGBToRGB565(255, 255, 255) {
		t.Errorf("Bottom-right pixel = 0x%04X, want 0x%04X (white)", brPixel, RGBToRGB565(255, 255, 255))
	}
}

func TestImageToRGB565Buffer(t *testing.T) {
	// Create a 480x320 image with a solid red color
	img := image.NewRGBA(image.Rect(0, 0, DefaultDisplayWidth, DefaultDisplayHeight))
	for y := 0; y < DefaultDisplayHeight; y++ {
		for x := 0; x < DefaultDisplayWidth; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	buffer := ImageToRGB565Buffer(img)

	if len(buffer) != DefaultBufferSize {
		t.Errorf("Buffer length = %d, want %d", len(buffer), DefaultBufferSize)
	}

	// Check first pixel is red
	if buffer[0] != 0xF8 || buffer[1] != 0x00 {
		t.Errorf("First pixel = [0x%02X, 0x%02X], want [0xF8, 0x00]", buffer[0], buffer[1])
	}
}

func TestImageToRGB565BufferBE(t *testing.T) {
	// Create a 10x10 image with green color
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{0, 255, 0, 255})
		}
	}

	buffer := ImageToRGB565BufferBE(img)

	expectedSize := 10 * 10 * BytesPerPixel
	if len(buffer) != expectedSize {
		t.Errorf("Buffer length = %d, want %d", len(buffer), expectedSize)
	}

	// Check first pixel is green (0x07E0 in big endian)
	if buffer[0] != 0x07 || buffer[1] != 0xE0 {
		t.Errorf("First pixel = [0x%02X, 0x%02X], want [0x07, 0xE0]", buffer[0], buffer[1])
	}
}

func TestImageToRGB565BufferLE(t *testing.T) {
	// Create a 10x10 image with green color
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{0, 255, 0, 255})
		}
	}

	buffer := ImageToRGB565BufferLE(img)

	expectedSize := 10 * 10 * BytesPerPixel
	if len(buffer) != expectedSize {
		t.Errorf("Buffer length = %d, want %d", len(buffer), expectedSize)
	}

	// Check first pixel is green (0x07E0 in little endian: 0xE0, 0x07)
	if buffer[0] != 0xE0 || buffer[1] != 0x07 {
		t.Errorf("First pixel = [0x%02X, 0x%02X], want [0xE0, 0x07]", buffer[0], buffer[1])
	}
}

func TestImageToRGB888Buffer(t *testing.T) {
	// Create a 10x10 image with blue color
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{0, 0, 255, 255})
		}
	}

	buffer := ImageToRGB888Buffer(img)

	expectedSize := 10 * 10 * 3 // RGB888 = 3 bytes per pixel
	if len(buffer) != expectedSize {
		t.Errorf("Buffer length = %d, want %d", len(buffer), expectedSize)
	}

	// Check first pixel is blue (R=0, G=0, B=255)
	if buffer[0] != 0 || buffer[1] != 0 || buffer[2] != 255 {
		t.Errorf("First pixel = [%d, %d, %d], want [0, 0, 255]", buffer[0], buffer[1], buffer[2])
	}
}

func TestCreateColorBarsBuffer(t *testing.T) {
	buffer := CreateColorBarsBuffer()

	if len(buffer) != DefaultBufferSize {
		t.Errorf("Buffer length = %d, want %d", len(buffer), DefaultBufferSize)
	}

	// Check first row is white (first color bar)
	whiteRGB565 := RGBToRGB565(255, 255, 255)
	firstPixel := binary.BigEndian.Uint16(buffer[0:2])
	if firstPixel != whiteRGB565 {
		t.Errorf("First pixel = 0x%04X, want 0x%04X (white)", firstPixel, whiteRGB565)
	}
}

func TestCreateSolidColorBufferWithSize(t *testing.T) {
	buffer := CreateSolidColorBufferWithSize(255, 0, 0, 100, 50)

	expectedSize := 100 * 50 * BytesPerPixel
	if len(buffer) != expectedSize {
		t.Errorf("Buffer length = %d, want %d", len(buffer), expectedSize)
	}

	// Check first pixel is red
	redRGB565 := RGBToRGB565(255, 0, 0)
	firstPixel := binary.BigEndian.Uint16(buffer[0:2])
	if firstPixel != redRGB565 {
		t.Errorf("First pixel = 0x%04X, want 0x%04X (red)", firstPixel, redRGB565)
	}
}

func TestCreateTestPatternBufferWithSize(t *testing.T) {
	width := 100
	height := 100
	buffer := CreateTestPatternBufferWithSize(width, height)

	expectedSize := width * height * BytesPerPixel
	if len(buffer) != expectedSize {
		t.Errorf("Buffer length = %d, want %d", len(buffer), expectedSize)
	}

	halfW := width / 2
	halfH := height / 2

	// Check top-left (red)
	tlIdx := 0
	tlPixel := binary.BigEndian.Uint16(buffer[tlIdx : tlIdx+2])
	if tlPixel != RGBToRGB565(255, 0, 0) {
		t.Errorf("Top-left pixel = 0x%04X, want red", tlPixel)
	}

	// Check top-right (green)
	trIdx := halfW * 2
	trPixel := binary.BigEndian.Uint16(buffer[trIdx : trIdx+2])
	if trPixel != RGBToRGB565(0, 255, 0) {
		t.Errorf("Top-right pixel = 0x%04X, want green", trPixel)
	}

	// Check bottom-left (blue)
	blIdx := halfH * width * 2
	blPixel := binary.BigEndian.Uint16(buffer[blIdx : blIdx+2])
	if blPixel != RGBToRGB565(0, 0, 255) {
		t.Errorf("Bottom-left pixel = 0x%04X, want blue", blPixel)
	}

	// Check bottom-right (white)
	brIdx := halfH*width*2 + halfW*2
	brPixel := binary.BigEndian.Uint16(buffer[brIdx : brIdx+2])
	if brPixel != RGBToRGB565(255, 255, 255) {
		t.Errorf("Bottom-right pixel = 0x%04X, want white", brPixel)
	}
}

func TestCreateColorBarsBufferWithSize(t *testing.T) {
	width := 100
	height := 80 // Divisible by 8 for even bars
	buffer := CreateColorBarsBufferWithSize(width, height)

	expectedSize := width * height * BytesPerPixel
	if len(buffer) != expectedSize {
		t.Errorf("Buffer length = %d, want %d", len(buffer), expectedSize)
	}

	// Check first row is white
	whiteRGB565 := RGBToRGB565(255, 255, 255)
	firstPixel := binary.BigEndian.Uint16(buffer[0:2])
	if firstPixel != whiteRGB565 {
		t.Errorf("First pixel = 0x%04X, want 0x%04X (white)", firstPixel, whiteRGB565)
	}

	// Check last row is black
	// Last row should be black
	blackRGB565 := RGBToRGB565(0, 0, 0)
	lastRowIdx := (height - 1) * width * 2
	lastPixel := binary.BigEndian.Uint16(buffer[lastRowIdx : lastRowIdx+2])
	if lastPixel != blackRGB565 {
		t.Errorf("Last pixel = 0x%04X, want 0x%04X (black)", lastPixel, blackRGB565)
	}
}

func TestImageScaling(t *testing.T) {
	// Test that ImageToRGB565Buffer handles images of different sizes
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	buffer := ImageToRGB565Buffer(img)

	// Should still produce default buffer size
	if len(buffer) != DefaultBufferSize {
		t.Errorf("Buffer length = %d, want %d", len(buffer), DefaultBufferSize)
	}
}

func TestImageWithOffset(t *testing.T) {
	// Test image with non-zero origin
	img := image.NewRGBA(image.Rect(10, 20, 110, 120))
	for y := 20; y < 120; y++ {
		for x := 10; x < 110; x++ {
			img.Set(x, y, color.RGBA{0, 0, 255, 255})
		}
	}

	buffer := ImageToRGB565BufferBE(img)

	expectedSize := 100 * 100 * BytesPerPixel
	if len(buffer) != expectedSize {
		t.Errorf("Buffer length = %d, want %d", len(buffer), expectedSize)
	}

	// Check first pixel is blue
	blueRGB565 := RGBToRGB565(0, 0, 255)
	firstPixel := binary.BigEndian.Uint16(buffer[0:2])
	if firstPixel != blueRGB565 {
		t.Errorf("First pixel = 0x%04X, want 0x%04X (blue)", firstPixel, blueRGB565)
	}
}

// Tests for device.go helper functions

func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"permission denied", "permission", true},
		{"PERMISSION DENIED", "permission", true},
		{"Permission Denied", "PERMISSION", true},
		{"access denied", "access denied", true},
		{"ACCESS DENIED", "access denied", true},
		{"some error message", "not found", false},
		{"", "test", false},
		{"test", "", true},
		{"LIBUSB_ERROR_ACCESS", "libusb_error_access", true},
		{"operation not permitted", "NOT PERMITTED", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			if got := containsIgnoreCase(tt.s, tt.substr); got != tt.want {
				t.Errorf("containsIgnoreCase(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

func TestBytesContains(t *testing.T) {
	tests := []struct {
		name string
		b    []byte
		sub  []byte
		want bool
	}{
		{"empty sub", []byte("hello"), []byte{}, true},
		{"exact match", []byte("hello"), []byte("hello"), true},
		{"prefix match", []byte("hello world"), []byte("hello"), true},
		{"suffix match", []byte("hello world"), []byte("world"), true},
		{"middle match", []byte("hello world"), []byte("lo wo"), true},
		{"no match", []byte("hello"), []byte("world"), false},
		{"sub longer than b", []byte("hi"), []byte("hello"), false},
		{"single byte match", []byte("abc"), []byte("b"), true},
		{"single byte no match", []byte("abc"), []byte("x"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bytesContains(tt.b, tt.sub); got != tt.want {
				t.Errorf("bytesContains(%v, %v) = %v, want %v", tt.b, tt.sub, got, tt.want)
			}
		})
	}
}

func TestIsPermissionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"access denied", errors.New("access denied"), true},
		{"permission denied", errors.New("permission denied"), true},
		{"LIBUSB_ERROR_ACCESS", errors.New("LIBUSB_ERROR_ACCESS"), true},
		{"operation not permitted", errors.New("operation not permitted"), true},
		{"insufficient permissions", errors.New("insufficient permissions"), true},
		{"uppercase ACCESS DENIED", errors.New("ACCESS DENIED"), true},
		{"mixed case Permission Denied", errors.New("Permission Denied"), true},
		{"unrelated error", errors.New("device not found"), false},
		{"timeout error", errors.New("operation timed out"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPermissionError(tt.err); got != tt.want {
				t.Errorf("isPermissionError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsDeviceBusyError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"device or resource busy", errors.New("device or resource busy"), true},
		{"resource busy", errors.New("resource busy"), true},
		{"LIBUSB_ERROR_BUSY", errors.New("LIBUSB_ERROR_BUSY"), true},
		{"code -6", errors.New("libusb: code -6"), true},
		{"uppercase RESOURCE BUSY", errors.New("RESOURCE BUSY"), true},
		{"unrelated error", errors.New("device not found"), false},
		{"permission error", errors.New("permission denied"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDeviceBusyError(tt.err); got != tt.want {
				t.Errorf("isDeviceBusyError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestPermissionFixInstructions(t *testing.T) {
	instructions := PermissionFixInstructions(0x1234, 0x5678)

	// Check that VID/PID are formatted correctly
	if !strings.Contains(instructions, "1234") {
		t.Error("Instructions should contain VID 1234")
	}
	if !strings.Contains(instructions, "5678") {
		t.Error("Instructions should contain PID 5678")
	}
	if !strings.Contains(instructions, "udev") {
		t.Error("Instructions should mention udev rules")
	}
	if !strings.Contains(instructions, "Zadig") {
		t.Error("Instructions should mention Zadig for Windows")
	}
}

func TestDeviceBusyInstructions(t *testing.T) {
	instructions := DeviceBusyInstructions()

	if !strings.Contains(instructions, "busy") {
		t.Error("Instructions should mention device busy")
	}
	if !strings.Contains(instructions, "another process") {
		t.Error("Instructions should mention another process")
	}
}

func TestNewDeviceWithID(t *testing.T) {
	dev := NewDeviceWithID(0x1234, 0x5678)

	if dev.targetVID != 0x1234 {
		t.Errorf("targetVID = 0x%04X, want 0x1234", dev.targetVID)
	}
	if dev.targetPID != 0x5678 {
		t.Errorf("targetPID = 0x%04X, want 0x5678", dev.targetPID)
	}
	if dev.Profile == nil {
		t.Error("Profile should not be nil")
	}
	if dev.Info == nil {
		t.Error("Info should not be nil")
	}
}

func TestNewDeviceWithSerial(t *testing.T) {
	dev := NewDeviceWithSerial(0x1234, 0x5678, "ABC123")

	if dev.targetVID != 0x1234 {
		t.Errorf("targetVID = 0x%04X, want 0x1234", dev.targetVID)
	}
	if dev.targetPID != 0x5678 {
		t.Errorf("targetPID = 0x%04X, want 0x5678", dev.targetPID)
	}
	if dev.targetSerial != "ABC123" {
		t.Errorf("targetSerial = %q, want %q", dev.targetSerial, "ABC123")
	}
}

func TestNewDeviceWithProfile(t *testing.T) {
	profile := &mockProfile{}
	dev := NewDeviceWithProfile(0xABCD, 0xEF01, profile)

	if dev.targetVID != 0xABCD {
		t.Errorf("targetVID = 0x%04X, want 0xABCD", dev.targetVID)
	}
	if dev.targetPID != 0xEF01 {
		t.Errorf("targetPID = 0x%04X, want 0xEF01", dev.targetPID)
	}
	if dev.Profile != profile {
		t.Error("Profile should match provided profile")
	}
}

func TestDevice_IsOpen(t *testing.T) {
	dev := NewDeviceWithID(0x1234, 0x5678)

	if dev.IsOpen() {
		t.Error("New device should not be open")
	}
}

func TestConstants(t *testing.T) {
	// Verify important constants
	if DefaultDisplayWidth != 480 {
		t.Errorf("DefaultDisplayWidth = %d, want 480", DefaultDisplayWidth)
	}
	if DefaultDisplayHeight != 320 {
		t.Errorf("DefaultDisplayHeight = %d, want 320", DefaultDisplayHeight)
	}
	if BytesPerPixel != 2 {
		t.Errorf("BytesPerPixel = %d, want 2", BytesPerPixel)
	}
	if DefaultBufferSize != 480*320*2 {
		t.Errorf("DefaultBufferSize = %d, want %d", DefaultBufferSize, 480*320*2)
	}
	if CBWLength != 31 {
		t.Errorf("CBWLength = %d, want 31", CBWLength)
	}
	if CSWLength != 13 {
		t.Errorf("CSWLength = %d, want 13", CSWLength)
	}
	if DirOut != 0x00 {
		t.Errorf("DirOut = 0x%02X, want 0x00", DirOut)
	}
	if DirIn != 0x80 {
		t.Errorf("DirIn = 0x%02X, want 0x80", DirIn)
	}
}
