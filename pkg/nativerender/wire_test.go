package nativerender

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWireBackgroundCacheAndRender(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	themeDir := t.TempDir()
	backgroundDir := filepath.Join(themeDir, "background")
	if err := os.MkdirAll(backgroundDir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := image.NewRGBA(image.Rect(0, 0, 32, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 32; x++ {
			src.SetRGBA(x, y, color.RGBA{R: uint8(x * 7), G: uint8(y * 13), B: 60, A: 255})
		}
	}
	file, err := os.Create(filepath.Join(backgroundDir, "frame_0001.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(file, src, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	render := New(&Theme{
		Layout:     "unsupported",
		Background: "#000000",
		BackgroundSequence: &BackgroundSequence{
			Path: "background", Pattern: "frame_%04d.jpg", Frames: 1,
			FPS: 1, Opacity: 1,
		},
		Performance: &Performance{},
	}, 32, 16)
	defer render.Close()
	if err := render.ConfigureWire(16, 32, 90); err != nil {
		t.Fatal(err)
	}
	if err := render.LoadBackgroundSequence(themeDir); err != nil {
		t.Fatal(err)
	}
	if render.OutputWidth() != 16 || render.OutputHeight() != 32 || !render.WireOutput() {
		t.Fatalf("wire output = %dx%d enabled=%v", render.OutputWidth(), render.OutputHeight(), render.WireOutput())
	}
	dst := image.NewRGBA(image.Rect(0, 0, 16, 32))
	render.RenderBackgroundInto(dst, time.Now())
	if dst.RGBAAt(8, 16).A != 255 {
		t.Fatal("wire background was not rendered")
	}
}
