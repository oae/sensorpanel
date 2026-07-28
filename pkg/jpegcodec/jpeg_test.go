package jpegcodec

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestReusableEncoderAndDecoder(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 32, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 32; x++ {
			src.SetRGBA(x, y, color.RGBA{R: uint8(x * 7), G: uint8(y * 13), B: 40, A: 255})
		}
	}
	encoder, err := NewEncoder(Config{Width: 32, Height: 16, Quality: 80, Backend: "stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	defer encoder.Close()
	payload, err := encoder.Encode(src)
	if err != nil {
		t.Fatal(err)
	}
	payload = append([]byte(nil), payload...)

	decoder, err := NewDecoder(Config{Width: 32, Height: 16, Backend: "stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	defer decoder.Close()
	dst := image.NewRGBA(src.Bounds())
	if err := decoder.DecodeInto(payload, dst); err != nil {
		t.Fatal(err)
	}
}

func TestRotateJPEGDimensions(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 32, 16))
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, src, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	rotated, err := RotateJPEG(encoded.Bytes(), 90)
	if err != nil {
		t.Fatal(err)
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(rotated))
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != 16 || config.Height != 32 {
		t.Fatalf("rotated dimensions = %dx%d, want 16x32", config.Width, config.Height)
	}
}
