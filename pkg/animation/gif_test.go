package animation

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestDecodeGIFScalesAndPreservesDelays(t *testing.T) {
	palette := color.Palette{color.Black, color.RGBA{R: 255, A: 255}}
	frame := image.NewPaletted(image.Rect(0, 0, 2, 1), palette)
	frame.SetColorIndex(0, 0, 1)

	var encoded bytes.Buffer
	err := gif.EncodeAll(&encoded, &gif.GIF{
		Image: []*image.Paletted{frame},
		Delay: []int{5},
		Config: image.Config{
			ColorModel: palette,
			Width:      2,
			Height:     1,
		},
	})
	if err != nil {
		t.Fatalf("encode fixture: %v", err)
	}

	animation, err := DecodeGIF(&encoded, 4, 4)
	if err != nil {
		t.Fatalf("DecodeGIF() error = %v", err)
	}
	if got := len(animation.Frames); got != 1 {
		t.Fatalf("frame count = %d, want 1", got)
	}
	if got := animation.Frames[0].Bounds(); got != image.Rect(0, 0, 4, 4) {
		t.Fatalf("frame bounds = %v, want 4x4", got)
	}
	if got := animation.Delays[0]; got != 50*time.Millisecond {
		t.Fatalf("delay = %v, want 50ms", got)
	}

	// A 2:1 source fitted into a square should have one black row above and below.
	if got := animation.Frames[0].RGBAAt(0, 0); got != (color.RGBA{A: 255}) {
		t.Fatalf("letterbox pixel = %#v, want opaque black", got)
	}
	if got := animation.Frames[0].RGBAAt(0, 1); got.R != 255 {
		t.Fatalf("scaled content pixel = %#v, want red", got)
	}
}

func TestDecodeGIFRejectsInvalidSize(t *testing.T) {
	if _, err := DecodeGIF(bytes.NewReader(nil), 0, 320); err == nil {
		t.Fatal("DecodeGIF() accepted zero width")
	}
}

func TestLoadGIFFromURL(t *testing.T) {
	palette := color.Palette{color.Black, color.White}
	frame := image.NewPaletted(image.Rect(0, 0, 1, 1), palette)
	frame.SetColorIndex(0, 0, 1)

	var encoded bytes.Buffer
	if err := gif.EncodeAll(&encoded, &gif.GIF{
		Image:  []*image.Paletted{frame},
		Delay:  []int{10},
		Config: image.Config{ColorModel: palette, Width: 1, Height: 1},
	}); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/gif")
		_, _ = w.Write(encoded.Bytes())
	}))
	defer server.Close()

	animation, err := LoadGIF(server.URL, 2, 2)
	if err != nil {
		t.Fatalf("LoadGIF() error = %v", err)
	}
	if got := len(animation.Frames); got != 1 {
		t.Fatalf("frame count = %d, want 1", got)
	}
}

func TestLoadImagePNG(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 1, 2))
	source.Set(0, 0, color.RGBA{G: 255, A: 255})

	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}

	path := t.TempDir() + "/image.png"
	if err := os.WriteFile(path, encoded.Bytes(), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	img, err := LoadImage(path, 4, 4)
	if err != nil {
		t.Fatalf("LoadImage() error = %v", err)
	}
	if got := img.Bounds(); got != image.Rect(0, 0, 4, 4) {
		t.Fatalf("image bounds = %v, want 4x4", got)
	}
	if got := img.RGBAAt(0, 0); got != (color.RGBA{A: 255}) {
		t.Fatalf("letterbox pixel = %#v, want opaque black", got)
	}
	if got := img.RGBAAt(1, 2); got != (color.RGBA{A: 255}) {
		t.Fatalf("transparent content pixel = %#v, want opaque black", got)
	}
}
