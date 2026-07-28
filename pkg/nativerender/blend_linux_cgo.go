//go:build linux && cgo

package nativerender

/*
#cgo CFLAGS: -O3
#include <stddef.h>
#include <stdint.h>

static void sensorpanel_blend_rgba(uint8_t * restrict dst, const uint8_t * restrict src, size_t pixels) {
  #pragma GCC ivdep
  for (size_t i = 0; i < pixels; ++i) {
    const size_t offset = i * 4;
    const unsigned int alpha = src[offset + 3];
    const unsigned int inverse = 255 - alpha;
    unsigned int red = src[offset] + (dst[offset] * inverse + 127) / 255;
    unsigned int green = src[offset + 1] + (dst[offset + 1] * inverse + 127) / 255;
    unsigned int blue = src[offset + 2] + (dst[offset + 2] * inverse + 127) / 255;
    dst[offset] = (uint8_t)(red > 255 ? 255 : red);
    dst[offset + 1] = (uint8_t)(green > 255 ? 255 : green);
    dst[offset + 2] = (uint8_t)(blue > 255 ? 255 : blue);
    dst[offset + 3] = 255;
  }
}
*/
import "C"

import "unsafe"

func blendRGBA(dst, src []byte) {
	if len(dst) == 0 || len(dst) != len(src) {
		return
	}
	C.sensorpanel_blend_rgba(
		(*C.uint8_t)(unsafe.Pointer(&dst[0])),
		(*C.uint8_t)(unsafe.Pointer(&src[0])),
		C.size_t(len(dst)/4),
	)
}
