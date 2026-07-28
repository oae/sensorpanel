// Package jpegcodec provides a selectable JPEG encoder for USB JPEG panels.
package jpegcodec

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
)

// Encode writes img as JPEG. encoder may be auto, stdlib, or turbo.
// auto uses libjpeg-turbo when this binary was built with -tags turbojpeg.
func Encode(dst *bytes.Buffer, img image.Image, quality int, encoder string) (string, error) {
	if quality < 1 || quality > 100 {
		return "", fmt.Errorf("JPEG quality must be between 1 and 100")
	}
	if encoder == "" || encoder == "auto" {
		if turboAvailable() {
			if err := encodeTurbo(dst, img, quality); err == nil {
				return "turbo", nil
			}
		}
		encoder = "stdlib"
	}
	switch encoder {
	case "stdlib":
		if err := jpeg.Encode(dst, img, &jpeg.Options{Quality: quality}); err != nil {
			return "", err
		}
		return "stdlib", nil
	case "turbo":
		if !turboAvailable() {
			return "", fmt.Errorf("TurboJPEG is unavailable; rebuild with -tags turbojpeg")
		}
		if err := encodeTurbo(dst, img, quality); err != nil {
			return "", err
		}
		return "turbo", nil
	default:
		return "", fmt.Errorf("unknown JPEG encoder %q", encoder)
	}
}
