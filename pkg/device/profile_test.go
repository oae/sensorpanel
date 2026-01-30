package device

import (
	"image"
	"image/color"
	"testing"
)

func TestColorFormat_String(t *testing.T) {
	tests := []struct {
		format ColorFormat
		want   string
	}{
		{RGB565, "RGB565"},
		{RGB888, "RGB888"},
		{ColorFormat(99), "Unknown"},
	}

	for _, tt := range tests {
		got := tt.format.String()
		if got != tt.want {
			t.Errorf("ColorFormat(%d).String() = %q, want %q", tt.format, got, tt.want)
		}
	}
}

func TestColorFormat_BytesPerPixel(t *testing.T) {
	tests := []struct {
		format ColorFormat
		want   int
	}{
		{RGB565, 2},
		{RGB888, 3},
		{ColorFormat(99), 2}, // default
	}

	for _, tt := range tests {
		got := tt.format.BytesPerPixel()
		if got != tt.want {
			t.Errorf("ColorFormat(%d).BytesPerPixel() = %d, want %d", tt.format, got, tt.want)
		}
	}
}

func TestByteOrder_String(t *testing.T) {
	tests := []struct {
		order ByteOrder
		want  string
	}{
		{BigEndian, "big-endian"},
		{LittleEndian, "little-endian"},
		{ByteOrder(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.order.String()
		if got != tt.want {
			t.Errorf("ByteOrder(%d).String() = %q, want %q", tt.order, got, tt.want)
		}
	}
}

func TestProtocolType_String(t *testing.T) {
	tests := []struct {
		proto ProtocolType
		want  string
	}{
		{ProtocolSCSI, "SCSI"},
		{ProtocolBulk, "Bulk"},
		{ProtocolType(99), "Unknown"},
	}

	for _, tt := range tests {
		got := tt.proto.String()
		if got != tt.want {
			t.Errorf("ProtocolType(%d).String() = %q, want %q", tt.proto, got, tt.want)
		}
	}
}

func TestGetInfo(t *testing.T) {
	profile := &QTKeJiProfile{}
	info := GetInfo(profile)

	if info.ID != "qtkeji" {
		t.Errorf("GetInfo().ID = %q, want %q", info.ID, "qtkeji")
	}
	if info.Name != "QTKeJi USB Display" {
		t.Errorf("GetInfo().Name = %q, want %q", info.Name, "QTKeJi USB Display")
	}
	if info.Width != 480 {
		t.Errorf("GetInfo().Width = %d, want %d", info.Width, 480)
	}
	if info.Height != 320 {
		t.Errorf("GetInfo().Height = %d, want %d", info.Height, 320)
	}
	if info.ColorFormat != RGB565 {
		t.Errorf("GetInfo().ColorFormat = %v, want %v", info.ColorFormat, RGB565)
	}
	if info.ByteOrder != BigEndian {
		t.Errorf("GetInfo().ByteOrder = %v, want %v", info.ByteOrder, BigEndian)
	}
	if len(info.VendorIDs) != 1 || info.VendorIDs[0] != 0x1908 {
		t.Errorf("GetInfo().VendorIDs = %v, want [0x1908]", info.VendorIDs)
	}
	if len(info.ProductIDs) != 2 {
		t.Errorf("GetInfo().ProductIDs = %v, want 2 elements", info.ProductIDs)
	}
}

// mockProfile is a simple mock for testing interface usage
type mockProfile struct{}

func (m *mockProfile) ID() string                                     { return "mock" }
func (m *mockProfile) Name() string                                   { return "Mock Device" }
func (m *mockProfile) Description() string                            { return "Mock description" }
func (m *mockProfile) Matches(vid, pid uint16) bool                   { return vid == 0x1234 && pid == 0x5678 }
func (m *mockProfile) VendorIDs() []uint16                            { return []uint16{0x1234} }
func (m *mockProfile) ProductIDs() []uint16                           { return []uint16{0x5678} }
func (m *mockProfile) Width() int                                     { return 320 }
func (m *mockProfile) Height() int                                    { return 240 }
func (m *mockProfile) ColorFormat() ColorFormat                       { return RGB888 }
func (m *mockProfile) ByteOrder() ByteOrder                           { return LittleEndian }
func (m *mockProfile) BufferSize() int                                { return 320 * 240 * 3 }
func (m *mockProfile) MaxBrightness() int                             { return 10 }
func (m *mockProfile) ProtocolType() ProtocolType                     { return ProtocolBulk }
func (m *mockProfile) BlitCommand(x, y, w, h int, dataLen int) []byte { return nil }
func (m *mockProfile) BacklightCommand(level int) []byte              { return nil }
func (m *mockProfile) ParseResponse(data []byte) error                { return nil }
func (m *mockProfile) ConvertImage(img image.Image) []byte            { return nil }

func TestDeviceProfileInterface(t *testing.T) {
	// Verify that mockProfile satisfies the interface
	var _ DeviceProfile = (*mockProfile)(nil)

	profile := &mockProfile{}
	info := GetInfo(profile)

	if info.ID != "mock" {
		t.Errorf("GetInfo().ID = %q, want %q", info.ID, "mock")
	}
	if info.ColorFormat != RGB888 {
		t.Errorf("GetInfo().ColorFormat = %v, want %v", info.ColorFormat, RGB888)
	}
	if info.ByteOrder != LittleEndian {
		t.Errorf("GetInfo().ByteOrder = %v, want %v", info.ByteOrder, LittleEndian)
	}
}

// Helper to create a test image
func createTestImage(width, height int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}
