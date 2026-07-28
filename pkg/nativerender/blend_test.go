package nativerender

import (
	"image"
	"testing"
)

func TestCompositeStraightAlpha(t *testing.T) {
	dst := image.NewRGBA(image.Rect(0, 0, 3, 1))
	overlay := image.NewRGBA(dst.Bounds())
	copy(dst.Pix, []byte{20, 40, 60, 255, 20, 40, 60, 255, 20, 40, 60, 255})
	copy(overlay.Pix, []byte{0, 0, 0, 0, 200, 100, 50, 128, 200, 100, 50, 255})
	render := &Renderer{}
	render.Composite(dst, overlay)
	if got := dst.RGBAAt(0, 0); got.R != 20 || got.G != 40 || got.B != 60 {
		t.Fatalf("transparent pixel = %v", got)
	}
	if got := dst.RGBAAt(1, 0); got.R != 210 || got.G != 120 || got.B != 80 {
		t.Fatalf("half-alpha pixel = %v", got)
	}
	if got := dst.RGBAAt(2, 0); got.R != 200 || got.G != 100 || got.B != 50 {
		t.Fatalf("opaque pixel = %v", got)
	}
}
