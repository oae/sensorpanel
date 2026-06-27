package device

import (
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

func TestNewGenericProfile(t *testing.T) {
	p := NewGenericProfile(0x1234, 0x5678)

	if p.vendorID != 0x1234 {
		t.Errorf("vendorID = %04X, want 1234", p.vendorID)
	}
	if p.productID != 0x5678 {
		t.Errorf("productID = %04X, want 5678", p.productID)
	}
	if p.width != 480 {
		t.Errorf("width = %d, want 480", p.width)
	}
	if p.height != 320 {
		t.Errorf("height = %d, want 320", p.height)
	}
}

func TestNewGenericProfileWithSize(t *testing.T) {
	p := NewGenericProfileWithSize(0x1234, 0x5678, 800, 480)

	if p.vendorID != 0x1234 {
		t.Errorf("vendorID = %04X, want 1234", p.vendorID)
	}
	if p.productID != 0x5678 {
		t.Errorf("productID = %04X, want 5678", p.productID)
	}
	if p.width != 800 {
		t.Errorf("width = %d, want 800", p.width)
	}
	if p.height != 480 {
		t.Errorf("height = %d, want 480", p.height)
	}
}

func TestGenericProfile_Identity(t *testing.T) {
	p := NewGenericProfile(0x1234, 0x5678)

	if p.ID() != "generic" {
		t.Errorf("ID() = %q, want %q", p.ID(), "generic")
	}

	expectedName := "Unknown Device (1234:5678)"
	if p.Name() != expectedName {
		t.Errorf("Name() = %q, want %q", p.Name(), expectedName)
	}

	if p.Description() == "" {
		t.Error("Description() should not be empty")
	}
}

func TestGenericProfile_Matches(t *testing.T) {
	p := NewGenericProfile(0x1234, 0x5678)

	tests := []struct {
		vid, pid uint16
		want     bool
	}{
		{0x1234, 0x5678, true},
		{0x1234, 0x0000, false},
		{0x0000, 0x5678, false},
		{0x0000, 0x0000, false},
	}

	for _, tt := range tests {
		got := p.Matches(tt.vid, tt.pid)
		if got != tt.want {
			t.Errorf("Matches(%04x, %04x) = %v, want %v", tt.vid, tt.pid, got, tt.want)
		}
	}
}

func TestGenericProfile_VIDPIDs(t *testing.T) {
	p := NewGenericProfile(0xABCD, 0xEF01)

	vids := p.VendorIDs()
	if len(vids) != 1 || vids[0] != 0xABCD {
		t.Errorf("VendorIDs() = %v, want [0xABCD]", vids)
	}

	pids := p.ProductIDs()
	if len(pids) != 1 || pids[0] != 0xEF01 {
		t.Errorf("ProductIDs() = %v, want [0xEF01]", pids)
	}
}

func TestGenericProfile_DisplayProperties(t *testing.T) {
	p := NewGenericProfileWithSize(0x1234, 0x5678, 640, 480)

	if p.Width() != 640 {
		t.Errorf("Width() = %d, want 640", p.Width())
	}

	if p.Height() != 480 {
		t.Errorf("Height() = %d, want 480", p.Height())
	}

	if p.ColorFormat() != RGB565 {
		t.Errorf("ColorFormat() = %v, want RGB565", p.ColorFormat())
	}

	if p.ByteOrder() != BigEndian {
		t.Errorf("ByteOrder() = %v, want BigEndian", p.ByteOrder())
	}

	expectedSize := 640 * 480 * 2
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

func TestGenericProfile_BlitCommand(t *testing.T) {
	p := NewGenericProfile(0x1234, 0x5678)

	cmd := p.BlitCommand(12, 34, 56, 78, 56*78*2)

	// CBW should be 31 bytes
	if len(cmd) != 31 {
		t.Errorf("BlitCommand() length = %d, want 31", len(cmd))
	}

	// Check CBW signature
	if string(cmd[0:4]) != "USBC" {
		t.Errorf("CBW signature = %q, want %q", string(cmd[0:4]), "USBC")
	}

	// Check command block
	if cmd[15] != 0xCD {
		t.Errorf("cmd[15] = %02x, want 0xCD (vendor prefix)", cmd[15])
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

func TestGenericProfile_BacklightCommand(t *testing.T) {
	p := NewGenericProfile(0x1234, 0x5678)

	tests := []struct {
		level     int
		wantLevel int
	}{
		{0, 0},
		{7, 7},
		{3, 3},
		{-1, 0},
		{10, 7},
	}

	for _, tt := range tests {
		cmd := p.BacklightCommand(tt.level)

		if len(cmd) != 31 {
			t.Errorf("BacklightCommand(%d) length = %d, want 31", tt.level, len(cmd))
		}

		// Check brightness level
		if int(cmd[24]) != tt.wantLevel {
			t.Errorf("BacklightCommand(%d) brightness byte = %d, want %d", tt.level, cmd[24], tt.wantLevel)
		}
	}
}

func TestGenericProfile_ParseResponse(t *testing.T) {
	p := NewGenericProfile(0x1234, 0x5678)

	// Valid CSW
	validCSW := make([]byte, 13)
	copy(validCSW[0:4], "USBS")
	validCSW[12] = 0

	if err := p.ParseResponse(validCSW); err != nil {
		t.Errorf("ParseResponse(valid CSW) = %v, want nil", err)
	}

	// Short CSW
	if err := p.ParseResponse(make([]byte, 5)); err == nil {
		t.Error("ParseResponse(short CSW) = nil, want error")
	}

	// Invalid signature
	invalidSig := make([]byte, 13)
	copy(invalidSig[0:4], "XXXX")
	if err := p.ParseResponse(invalidSig); err == nil {
		t.Error("ParseResponse(invalid signature) = nil, want error")
	}

	// Error status
	errorCSW := make([]byte, 13)
	copy(errorCSW[0:4], "USBS")
	errorCSW[12] = 1
	if err := p.ParseResponse(errorCSW); err == nil {
		t.Error("ParseResponse(error status) = nil, want error")
	}
}

func TestGenericProfile_ConvertImage(t *testing.T) {
	p := NewGenericProfileWithSize(0x1234, 0x5678, 100, 100)

	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{0, 0, 255, 255}) // Blue
		}
	}

	buffer := p.ConvertImage(img)

	expectedSize := 100 * 100 * 2
	if len(buffer) != expectedSize {
		t.Errorf("ConvertImage() buffer size = %d, want %d", len(buffer), expectedSize)
	}

	// Blue in RGB565: 0x001F
	// Big-endian: 0x00, 0x1F
	if buffer[0] != 0x00 || buffer[1] != 0x1F {
		t.Errorf("ConvertImage() first pixel = %02x%02x, want 001F (blue)", buffer[0], buffer[1])
	}
}

func TestGenericProfile_buildCBW(t *testing.T) {
	p := NewGenericProfile(0x1234, 0x5678)

	cmd := make([]byte, 16)
	cmd[0] = 0xAB

	cbw := p.buildCBW(cmd, 500, 0x80)

	if len(cbw) != 31 {
		t.Errorf("buildCBW() length = %d, want 31", len(cbw))
	}

	if string(cbw[0:4]) != "USBC" {
		t.Errorf("buildCBW() signature = %q, want %q", string(cbw[0:4]), "USBC")
	}

	if cbw[12] != 0x80 {
		t.Errorf("buildCBW() direction = %02X, want 0x80", cbw[12])
	}

	if cbw[15] != 0xAB {
		t.Errorf("buildCBW() command[0] = %02X, want 0xAB", cbw[15])
	}
}

func TestGenericProfile_ImplementsInterface(t *testing.T) {
	var _ DeviceProfile = (*GenericProfile)(nil)
}
