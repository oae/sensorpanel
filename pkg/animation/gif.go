// Package animation loads animated images for display on a sensor panel.
package animation

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const maxRemoteImageSize = 32 << 20 // 32 MiB

// GIF contains fully composited, panel-sized frames and their display durations.
type GIF struct {
	Frames []*image.RGBA
	Delays []time.Duration
}

// LoadImage decodes the first frame of a PNG, JPEG, or GIF from a local file
// or HTTP(S) URL and scales it to fit within width x height.
func LoadImage(source string, width, height int) (*image.RGBA, error) {
	data, err := readSource(source)
	if err != nil {
		return nil, err
	}

	decoded, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	if format != "png" && format != "jpeg" && format != "gif" {
		return nil, fmt.Errorf("unsupported image format %q (supported: PNG, JPEG, GIF)", format)
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid output size %dx%d", width, height)
	}

	return fitRGBA(decoded, width, height), nil
}

// LoadGIF decodes a local file or HTTP(S) URL and scales it to fit within
// width x height.
func LoadGIF(source string, width, height int) (*GIF, error) {
	data, err := readSource(source)
	if err != nil {
		return nil, err
	}
	return DecodeGIF(bytes.NewReader(data), width, height)
}

func readSource(source string) ([]byte, error) {
	if !strings.HasPrefix(source, "https://") && !strings.HasPrefix(source, "http://") {
		data, err := os.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("open image: %w", err)
		}
		return data, nil
	}

	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Get(source)
	if err != nil {
		return nil, fmt.Errorf("download image: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("download image: server returned %s", response.Status)
	}

	data, err := io.ReadAll(io.LimitReader(response.Body, maxRemoteImageSize+1))
	if err != nil {
		return nil, fmt.Errorf("download image: %w", err)
	}
	if len(data) > maxRemoteImageSize {
		return nil, fmt.Errorf("download image: file exceeds 32 MiB limit")
	}

	return data, nil
}

// DecodeGIF decodes and composites GIF frames, including disposal handling.
func DecodeGIF(r io.Reader, width, height int) (*GIF, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid output size %dx%d", width, height)
	}

	decoded, err := gif.DecodeAll(r)
	if err != nil {
		return nil, fmt.Errorf("decode GIF: %w", err)
	}
	if len(decoded.Image) == 0 {
		return nil, fmt.Errorf("GIF contains no frames")
	}

	canvasBounds := image.Rect(0, 0, decoded.Config.Width, decoded.Config.Height)
	canvas := image.NewRGBA(canvasBounds)
	var background color.Color = color.Transparent
	if decoded.BackgroundIndex < uint8(len(decoded.Image[0].Palette)) {
		background = decoded.Image[0].Palette[decoded.BackgroundIndex]
	}
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{background}, image.Point{}, draw.Src)

	result := &GIF{
		Frames: make([]*image.RGBA, 0, len(decoded.Image)),
		Delays: make([]time.Duration, 0, len(decoded.Image)),
	}

	for i, frame := range decoded.Image {
		var previous *image.RGBA
		if disposalAt(decoded, i) == gif.DisposalPrevious {
			previous = cloneRGBA(canvas)
		}

		draw.Draw(canvas, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
		result.Frames = append(result.Frames, fitRGBA(canvas, width, height))

		delay := time.Duration(decoded.Delay[i]) * 10 * time.Millisecond
		if delay <= 0 {
			delay = 100 * time.Millisecond
		}
		result.Delays = append(result.Delays, delay)

		switch disposalAt(decoded, i) {
		case gif.DisposalBackground:
			draw.Draw(canvas, frame.Bounds(), &image.Uniform{background}, image.Point{}, draw.Src)
		case gif.DisposalPrevious:
			canvas = previous
		}
	}

	return result, nil
}

func disposalAt(g *gif.GIF, i int) byte {
	if i < len(g.Disposal) {
		return g.Disposal[i]
	}
	return gif.DisposalNone
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return dst
}

// fitRGBA performs nearest-neighbor scaling while preserving aspect ratio.
// Unused space is filled black, which avoids stretching the animation.
func fitRGBA(src image.Image, width, height int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(dst, dst.Bounds(), image.Black, image.Point{}, draw.Src)

	srcBounds := src.Bounds()
	srcW, srcH := srcBounds.Dx(), srcBounds.Dy()
	scale := min(float64(width)/float64(srcW), float64(height)/float64(srcH))
	dstW := max(1, int(float64(srcW)*scale))
	dstH := max(1, int(float64(srcH)*scale))
	offsetX := (width - dstW) / 2
	offsetY := (height - dstH) / 2

	for y := 0; y < dstH; y++ {
		sy := srcBounds.Min.Y + y*srcH/dstH
		for x := 0; x < dstW; x++ {
			sx := srcBounds.Min.X + x*srcW/dstW
			// Panels use RGB565 and have no alpha channel. Flatten transparent
			// pixels onto black so hidden palette RGB values cannot appear as
			// remnants of an older frame.
			pixel := color.NRGBAModel.Convert(src.At(sx, sy)).(color.NRGBA)
			alpha := uint16(pixel.A)
			dst.SetRGBA(offsetX+x, offsetY+y, color.RGBA{
				R: uint8(uint16(pixel.R) * alpha / 255),
				G: uint8(uint16(pixel.G) * alpha / 255),
				B: uint8(uint16(pixel.B) * alpha / 255),
				A: 255,
			})
		}
	}

	return dst
}
