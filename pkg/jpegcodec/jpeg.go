// Package jpegcodec provides a selectable JPEG encoder for USB JPEG panels.
package jpegcodec

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
)

// Config describes a reusable JPEG encoder. Encoded returns a borrowed byte
// slice which remains valid until the next Encode call or Close.
type Config struct {
	Width   int
	Height  int
	Quality int
	Backend string
}

// Encoder encodes a fixed-size RGBA stream without rebuilding codec state for
// every frame. Implementations are intentionally single-goroutine.
type Encoder interface {
	Name() string
	Encode(*image.RGBA) ([]byte, error)
	Close() error
}

// Decoder decodes a fixed-size JPEG stream into a caller-owned RGBA buffer.
type Decoder interface {
	Name() string
	DecodeInto([]byte, *image.RGBA) error
	Close() error
}

// NewEncoder creates a reusable encoder. auto prefers TurboJPEG when the
// binary was built with -tags turbojpeg and falls back to the standard library.
func NewEncoder(config Config) (Encoder, error) {
	if config.Width <= 0 || config.Height <= 0 {
		return nil, fmt.Errorf("JPEG dimensions must be positive")
	}
	if config.Quality < 1 || config.Quality > 100 {
		return nil, fmt.Errorf("JPEG quality must be between 1 and 100")
	}
	if config.Backend == "" {
		config.Backend = "auto"
	}
	if config.Backend == "auto" && turboAvailable() {
		encoder, err := newTurboEncoder(config)
		if err == nil {
			return encoder, nil
		}
		config.Backend = "stdlib"
	}
	switch config.Backend {
	case "stdlib":
		return &stdlibEncoder{
			width:   config.Width,
			height:  config.Height,
			options: jpeg.Options{Quality: config.Quality},
		}, nil
	case "turbo":
		if !turboAvailable() {
			return nil, fmt.Errorf("TurboJPEG is unavailable; rebuild with -tags turbojpeg")
		}
		return newTurboEncoder(config)
	default:
		return nil, fmt.Errorf("unknown JPEG encoder %q", config.Backend)
	}
}

// NewDecoder creates a reusable decoder. Quality is ignored.
func NewDecoder(config Config) (Decoder, error) {
	if config.Width <= 0 || config.Height <= 0 {
		return nil, fmt.Errorf("JPEG dimensions must be positive")
	}
	if config.Backend == "" {
		config.Backend = "auto"
	}
	if config.Backend == "auto" && turboAvailable() {
		decoder, err := newTurboDecoder(config)
		if err == nil {
			return decoder, nil
		}
		config.Backend = "stdlib"
	}
	switch config.Backend {
	case "stdlib":
		return &stdlibDecoder{width: config.Width, height: config.Height}, nil
	case "turbo":
		if !turboAvailable() {
			return nil, fmt.Errorf("TurboJPEG is unavailable; rebuild with -tags turbojpeg")
		}
		return newTurboDecoder(config)
	default:
		return nil, fmt.Errorf("unknown JPEG decoder %q", config.Backend)
	}
}

type stdlibEncoder struct {
	width   int
	height  int
	options jpeg.Options
	buffer  bytes.Buffer
}

func (e *stdlibEncoder) Name() string { return "stdlib" }

func (e *stdlibEncoder) Encode(img *image.RGBA) ([]byte, error) {
	if err := validateRGBA(img, e.width, e.height); err != nil {
		return nil, err
	}
	e.buffer.Reset()
	if err := jpeg.Encode(&e.buffer, img, &e.options); err != nil {
		return nil, err
	}
	return e.buffer.Bytes(), nil
}

func (e *stdlibEncoder) Close() error {
	e.buffer.Reset()
	return nil
}

type stdlibDecoder struct {
	width  int
	height int
}

func (d *stdlibDecoder) Name() string { return "stdlib" }

func (d *stdlibDecoder) DecodeInto(data []byte, dst *image.RGBA) error {
	if err := validateRGBA(dst, d.width, d.height); err != nil {
		return err
	}
	src, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}
	if src.Bounds().Dx() != d.width || src.Bounds().Dy() != d.height {
		return fmt.Errorf("JPEG dimensions = %dx%d, expected %dx%d",
			src.Bounds().Dx(), src.Bounds().Dy(), d.width, d.height)
	}
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return nil
}

func (d *stdlibDecoder) Close() error { return nil }

// Encode writes img as JPEG. encoder may be auto, stdlib, or turbo.
// auto uses libjpeg-turbo when this binary was built with -tags turbojpeg.
func Encode(dst *bytes.Buffer, img image.Image, quality int, encoder string) (string, error) {
	rgba, ok := img.(*image.RGBA)
	if !ok || rgba.Bounds().Min != (image.Point{}) {
		bounds := img.Bounds()
		rgba = image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
		draw.Draw(rgba, rgba.Bounds(), img, bounds.Min, draw.Src)
	}
	reusable, err := NewEncoder(Config{
		Width:   rgba.Bounds().Dx(),
		Height:  rgba.Bounds().Dy(),
		Quality: quality,
		Backend: encoder,
	})
	if err != nil {
		return "", err
	}
	defer reusable.Close()
	payload, err := reusable.Encode(rgba)
	if err != nil {
		return "", err
	}
	_, _ = dst.Write(payload)
	return reusable.Name(), nil
}

func validateRGBA(img *image.RGBA, width, height int) error {
	if img == nil || len(img.Pix) == 0 {
		return fmt.Errorf("cannot encode empty RGBA image")
	}
	if img.Rect.Min != (image.Point{}) || img.Bounds().Dx() != width || img.Bounds().Dy() != height {
		return fmt.Errorf("JPEG RGBA dimensions = %v, expected (0,0)-(%d,%d)", img.Bounds(), width, height)
	}
	return nil
}

// RotateJPEG rotates a JPEG clockwise by a right angle. TurboJPEG performs a
// lossless DCT transform; the portable fallback decodes and re-encodes once.
func RotateJPEG(data []byte, degrees int) ([]byte, error) {
	degrees = ((degrees % 360) + 360) % 360
	if degrees == 0 {
		return append([]byte(nil), data...), nil
	}
	if degrees != 90 && degrees != 180 && degrees != 270 {
		return nil, fmt.Errorf("JPEG rotation must be 0, 90, 180, or 270")
	}
	if turboAvailable() {
		if transformed, err := rotateTurboJPEG(data, degrees); err == nil {
			return transformed, nil
		}
	}
	src, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	dst := rotateImage(src, degrees)
	var output bytes.Buffer
	if err := jpeg.Encode(&output, dst, &jpeg.Options{Quality: 95}); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func rotateImage(src image.Image, degrees int) *image.RGBA {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	dstWidth, dstHeight := width, height
	if degrees == 90 || degrees == 270 {
		dstWidth, dstHeight = height, width
	}
	dst := image.NewRGBA(image.Rect(0, 0, dstWidth, dstHeight))
	for y := 0; y < dstHeight; y++ {
		for x := 0; x < dstWidth; x++ {
			var sx, sy int
			switch degrees {
			case 90:
				sx, sy = y, height-1-x
			case 180:
				sx, sy = width-1-x, height-1-y
			case 270:
				sx, sy = width-1-y, x
			}
			c := color.RGBAModel.Convert(src.At(bounds.Min.X+sx, bounds.Min.Y+sy)).(color.RGBA)
			dst.SetRGBA(x, y, c)
		}
	}
	return dst
}
