//go:build turbojpeg && linux && cgo

package jpegcodec

/*
#cgo pkg-config: libturbojpeg
#include <stdlib.h>
#include <turbojpeg.h>

static int sensorpanel_tj_compress_rgba(unsigned char *src, int width, int height, int stride, int quality, unsigned char **out, unsigned long *out_size) {
  tjhandle handle = tjInitCompress();
  if (!handle) return -1;
  int rc = tjCompress2(handle, src, width, stride, height, TJPF_RGBA, out, out_size, TJSAMP_420, quality, TJFLAG_FASTDCT);
  tjDestroy(handle);
  return rc;
}
*/
import "C"

import (
	"bytes"
	"fmt"
	"image"
	"unsafe"
)

func turboAvailable() bool { return true }

func encodeTurbo(dst *bytes.Buffer, img image.Image, quality int) error {
	rgba, ok := img.(*image.RGBA)
	if !ok || rgba.Rect.Min.X != 0 || rgba.Rect.Min.Y != 0 {
		copy := image.NewRGBA(img.Bounds())
		for y := copy.Rect.Min.Y; y < copy.Rect.Max.Y; y++ {
			for x := copy.Rect.Min.X; x < copy.Rect.Max.X; x++ {
				copy.Set(x, y, img.At(x, y))
			}
		}
		rgba = copy
	}
	if len(rgba.Pix) == 0 {
		return fmt.Errorf("cannot encode empty RGBA image")
	}
	var output *C.uchar
	var outputSize C.ulong
	rc := C.sensorpanel_tj_compress_rgba(
		(*C.uchar)(unsafe.Pointer(&rgba.Pix[0])), C.int(rgba.Rect.Dx()), C.int(rgba.Rect.Dy()), C.int(rgba.Stride), C.int(quality), &output, &outputSize,
	)
	if rc != 0 {
		return fmt.Errorf("TurboJPEG encode failed")
	}
	defer C.tjFree(output)
	dst.Write(unsafe.Slice((*byte)(unsafe.Pointer(output)), int(outputSize)))
	return nil
}
