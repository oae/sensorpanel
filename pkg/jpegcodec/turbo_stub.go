//go:build !turbojpeg || !linux || !cgo

package jpegcodec

import (
	"bytes"
	"fmt"
	"image"
)

func turboAvailable() bool { return false }

func encodeTurbo(_ *bytes.Buffer, _ image.Image, _ int) error {
	return fmt.Errorf("TurboJPEG is unavailable")
}
