//go:build turbojpeg && linux && cgo

package jpegcodec

/*
#cgo pkg-config: libturbojpeg
#include <stdlib.h>
#include <string.h>
#include <turbojpeg.h>

typedef struct {
  tjhandle handle;
  unsigned char *output;
  unsigned long capacity;
} sensorpanel_tj_encoder;

typedef struct {
  tjhandle handle;
} sensorpanel_tj_decoder;

static sensorpanel_tj_encoder *sensorpanel_tj_encoder_new(int width, int height) {
  sensorpanel_tj_encoder *encoder = (sensorpanel_tj_encoder *)calloc(1, sizeof(sensorpanel_tj_encoder));
  if (!encoder) return NULL;
  encoder->handle = tjInitCompress();
  if (!encoder->handle) {
    free(encoder);
    return NULL;
  }
  encoder->capacity = tjBufSize(width, height, TJSAMP_420);
  encoder->output = (unsigned char *)tjAlloc(encoder->capacity);
  if (!encoder->output) {
    tjDestroy(encoder->handle);
    free(encoder);
    return NULL;
  }
  return encoder;
}

static int sensorpanel_tj_encoder_compress(sensorpanel_tj_encoder *encoder,
    unsigned char *src, int width, int height, int stride, int quality,
    unsigned long *output_size) {
  unsigned char *output = encoder->output;
  unsigned long size = encoder->capacity;
  int rc = tjCompress2(encoder->handle, src, width, stride, height, TJPF_RGBA,
      &output, &size, TJSAMP_420, quality, TJFLAG_FASTDCT | TJFLAG_NOREALLOC);
  if (rc == 0) *output_size = size;
  return rc;
}

static const char *sensorpanel_tj_encoder_error(sensorpanel_tj_encoder *encoder) {
  return encoder && encoder->handle ? tjGetErrorStr2(encoder->handle) : "TurboJPEG encoder unavailable";
}

static void sensorpanel_tj_encoder_free(sensorpanel_tj_encoder *encoder) {
  if (!encoder) return;
  if (encoder->output) tjFree(encoder->output);
  if (encoder->handle) tjDestroy(encoder->handle);
  free(encoder);
}

static sensorpanel_tj_decoder *sensorpanel_tj_decoder_new() {
  sensorpanel_tj_decoder *decoder = (sensorpanel_tj_decoder *)calloc(1, sizeof(sensorpanel_tj_decoder));
  if (!decoder) return NULL;
  decoder->handle = tjInitDecompress();
  if (!decoder->handle) {
    free(decoder);
    return NULL;
  }
  return decoder;
}

static int sensorpanel_tj_decoder_decompress(sensorpanel_tj_decoder *decoder,
    unsigned char *src, unsigned long src_size, unsigned char *dst,
    int expected_width, int expected_height, int stride) {
  int width = 0, height = 0, subsamp = 0, colorspace = 0;
  if (tjDecompressHeader3(decoder->handle, src, src_size, &width, &height, &subsamp, &colorspace) != 0)
    return -1;
  if (width != expected_width || height != expected_height)
    return -2;
  return tjDecompress2(decoder->handle, src, src_size, dst, width, stride,
      height, TJPF_RGBA, TJFLAG_FASTDCT | TJFLAG_FASTUPSAMPLE);
}

static const char *sensorpanel_tj_decoder_error(sensorpanel_tj_decoder *decoder) {
  return decoder && decoder->handle ? tjGetErrorStr2(decoder->handle) : "TurboJPEG decoder unavailable";
}

static void sensorpanel_tj_decoder_free(sensorpanel_tj_decoder *decoder) {
  if (!decoder) return;
  if (decoder->handle) tjDestroy(decoder->handle);
  free(decoder);
}

static int sensorpanel_tj_transform(unsigned char *src, unsigned long src_size,
    int operation, unsigned char **output, unsigned long *output_size) {
  tjhandle handle = tjInitTransform();
  if (!handle) return -1;
  tjtransform transform;
  memset(&transform, 0, sizeof(transform));
  transform.op = operation;
  int rc = tjTransform(handle, src, src_size, 1, output, output_size,
      &transform, TJFLAG_FASTDCT);
  tjDestroy(handle);
  return rc;
}
*/
import "C"

import (
	"fmt"
	"image"
	"runtime"
	"unsafe"
)

func turboAvailable() bool { return true }

type turboEncoder struct {
	handle  *C.sensorpanel_tj_encoder
	width   int
	height  int
	quality int
	closed  bool
}

type turboDecoder struct {
	handle *C.sensorpanel_tj_decoder
	width  int
	height int
	closed bool
}

func newTurboEncoder(config Config) (Encoder, error) {
	handle := C.sensorpanel_tj_encoder_new(C.int(config.Width), C.int(config.Height))
	if handle == nil {
		return nil, fmt.Errorf("create TurboJPEG encoder")
	}
	encoder := &turboEncoder{
		handle:  handle,
		width:   config.Width,
		height:  config.Height,
		quality: config.Quality,
	}
	runtime.SetFinalizer(encoder, func(e *turboEncoder) { _ = e.Close() })
	return encoder, nil
}

func (e *turboEncoder) Name() string { return "turbo" }

func (e *turboEncoder) Encode(img *image.RGBA) ([]byte, error) {
	if e.closed || e.handle == nil {
		return nil, fmt.Errorf("TurboJPEG encoder is closed")
	}
	if err := validateRGBA(img, e.width, e.height); err != nil {
		return nil, err
	}
	var outputSize C.ulong
	rc := C.sensorpanel_tj_encoder_compress(
		e.handle,
		(*C.uchar)(unsafe.Pointer(&img.Pix[0])),
		C.int(e.width),
		C.int(e.height),
		C.int(img.Stride),
		C.int(e.quality),
		&outputSize,
	)
	if rc != 0 {
		return nil, fmt.Errorf("TurboJPEG encode failed: %s", C.GoString(C.sensorpanel_tj_encoder_error(e.handle)))
	}
	runtime.KeepAlive(img)
	return unsafe.Slice((*byte)(unsafe.Pointer(e.handle.output)), int(outputSize)), nil
}

func (e *turboEncoder) Close() error {
	if e == nil || e.closed {
		return nil
	}
	e.closed = true
	runtime.SetFinalizer(e, nil)
	C.sensorpanel_tj_encoder_free(e.handle)
	e.handle = nil
	return nil
}

func newTurboDecoder(config Config) (Decoder, error) {
	handle := C.sensorpanel_tj_decoder_new()
	if handle == nil {
		return nil, fmt.Errorf("create TurboJPEG decoder")
	}
	decoder := &turboDecoder{handle: handle, width: config.Width, height: config.Height}
	runtime.SetFinalizer(decoder, func(d *turboDecoder) { _ = d.Close() })
	return decoder, nil
}

func (d *turboDecoder) Name() string { return "turbo" }

func (d *turboDecoder) DecodeInto(data []byte, dst *image.RGBA) error {
	if d.closed || d.handle == nil {
		return fmt.Errorf("TurboJPEG decoder is closed")
	}
	if len(data) == 0 {
		return fmt.Errorf("cannot decode empty JPEG")
	}
	if err := validateRGBA(dst, d.width, d.height); err != nil {
		return err
	}
	rc := C.sensorpanel_tj_decoder_decompress(
		d.handle,
		(*C.uchar)(unsafe.Pointer(&data[0])),
		C.ulong(len(data)),
		(*C.uchar)(unsafe.Pointer(&dst.Pix[0])),
		C.int(d.width),
		C.int(d.height),
		C.int(dst.Stride),
	)
	if rc == -2 {
		return fmt.Errorf("TurboJPEG dimensions do not match %dx%d", d.width, d.height)
	}
	if rc != 0 {
		return fmt.Errorf("TurboJPEG decode failed: %s", C.GoString(C.sensorpanel_tj_decoder_error(d.handle)))
	}
	runtime.KeepAlive(data)
	runtime.KeepAlive(dst)
	return nil
}

func (d *turboDecoder) Close() error {
	if d == nil || d.closed {
		return nil
	}
	d.closed = true
	runtime.SetFinalizer(d, nil)
	C.sensorpanel_tj_decoder_free(d.handle)
	d.handle = nil
	return nil
}

func rotateTurboJPEG(data []byte, degrees int) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("cannot transform empty JPEG")
	}
	operation := C.int(C.TJXOP_NONE)
	switch degrees {
	case 90:
		operation = C.TJXOP_ROT90
	case 180:
		operation = C.TJXOP_ROT180
	case 270:
		operation = C.TJXOP_ROT270
	}
	var output *C.uchar
	var outputSize C.ulong
	if rc := C.sensorpanel_tj_transform(
		(*C.uchar)(unsafe.Pointer(&data[0])),
		C.ulong(len(data)),
		operation,
		&output,
		&outputSize,
	); rc != 0 {
		return nil, fmt.Errorf("TurboJPEG transform failed")
	}
	defer C.tjFree(output)
	return C.GoBytes(unsafe.Pointer(output), C.int(outputSize)), nil
}
