//go:build !turbojpeg || !linux || !cgo

package jpegcodec

import (
	"fmt"
)

func turboAvailable() bool { return false }

func newTurboEncoder(_ Config) (Encoder, error) {
	return nil, fmt.Errorf("TurboJPEG is unavailable")
}

func newTurboDecoder(_ Config) (Decoder, error) {
	return nil, fmt.Errorf("TurboJPEG is unavailable")
}

func rotateTurboJPEG(_ []byte, _ int) ([]byte, error) {
	return nil, fmt.Errorf("TurboJPEG is unavailable")
}
