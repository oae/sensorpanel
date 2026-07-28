package device

import (
	"image"
	"image/color"
	"testing"
)

func TestThermalrightTrofeoProfile(t *testing.T) {
	p := &ThermalrightTrofeoProfile{}

	if got := p.ID(); got != "thermalright_trofeo_916" {
		t.Fatalf("ID() = %q", got)
	}
	if !p.Matches(0x0416, 0x5408) {
		t.Fatal("expected profile to match 0416:5408")
	}
	if p.Matches(0x1908, 0x0102) {
		t.Fatal("did not expect profile to match QTKeJi device")
	}
	if got, want := p.Width(), 1920; got != want {
		t.Fatalf("Width() = %d, want %d", got, want)
	}
	if got, want := p.Height(), 462; got != want {
		t.Fatalf("Height() = %d, want %d", got, want)
	}
	if got := p.ColorFormat(); got != RGB888 {
		t.Fatalf("ColorFormat() = %v, want RGB888", got)
	}
	if got := p.ProtocolType(); got != ProtocolLYBulk {
		t.Fatalf("ProtocolType() = %v, want ProtocolLYBulk", got)
	}
	if got, want := p.BufferSize(), 1920*462*3; got != want {
		t.Fatalf("BufferSize() = %d, want %d", got, want)
	}
}

func TestThermalrightTrofeoProfileConvertImage(t *testing.T) {
	p := &ThermalrightTrofeoProfile{}
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 12, G: 34, B: 56, A: 255})

	buffer := p.ConvertImage(img)
	if got, want := len(buffer), p.BufferSize(); got != want {
		t.Fatalf("ConvertImage() len = %d, want %d", got, want)
	}
	if buffer[0] != 12 || buffer[1] != 34 || buffer[2] != 56 {
		t.Fatalf("first pixel = [%d %d %d], want [12 34 56]", buffer[0], buffer[1], buffer[2])
	}
}

func TestThermalrightTrofeoProfileRegistered(t *testing.T) {
	p := FindByVIDPID(0x0416, 0x5408)
	if p == nil {
		t.Fatal("FindByVIDPID(0416:5408) returned nil")
	}
	if got := p.ID(); got != "thermalright_trofeo_916" {
		t.Fatalf("registered profile ID = %q", got)
	}
}
