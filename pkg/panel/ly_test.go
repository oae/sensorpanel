package panel

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestBuildLYPacket(t *testing.T) {
	payload := make([]byte, lyChunkDataSize+10)
	for i := range payload {
		payload[i] = byte(i)
	}

	packet := buildLYPacket(payload)
	if got, want := len(packet), lyPadChunkMultiple*lyChunkSize; got != want {
		t.Fatalf("packet len = %d, want %d", got, want)
	}

	if packet[0] != 0x01 || packet[1] != 0xff {
		t.Fatalf("first chunk header = %02x %02x, want 01 ff", packet[0], packet[1])
	}
	if got := binary.LittleEndian.Uint32(packet[2:6]); got != uint32(len(payload)) {
		t.Fatalf("payload size header = %d, want %d", got, len(payload))
	}
	if got := binary.LittleEndian.Uint16(packet[6:8]); got != lyChunkDataSize {
		t.Fatalf("first chunk data len = %d, want %d", got, lyChunkDataSize)
	}
	if got := packet[8]; got != lyChunkCommand {
		t.Fatalf("chunk command = %d, want %d", got, lyChunkCommand)
	}
	if got := binary.LittleEndian.Uint16(packet[9:11]); got != 2 {
		t.Fatalf("chunk count = %d, want 2", got)
	}
	if got := binary.LittleEndian.Uint16(packet[11:13]); got != 0 {
		t.Fatalf("first chunk index = %d, want 0", got)
	}

	second := lyChunkSize
	if got := binary.LittleEndian.Uint16(packet[second+6 : second+8]); got != 10 {
		t.Fatalf("last chunk data len = %d, want 10", got)
	}
	if got := binary.LittleEndian.Uint16(packet[second+11 : second+13]); got != 1 {
		t.Fatalf("second chunk index = %d, want 1", got)
	}
	if got := packet[second+lyChunkHeaderSize]; got != payload[lyChunkDataSize] {
		t.Fatalf("second chunk first payload byte = %d, want %d", got, payload[lyChunkDataSize])
	}
}

func TestBuildLYPacketAddsEmptyLastChunkForExactMultiple(t *testing.T) {
	payload := make([]byte, lyChunkDataSize)
	packet := buildLYPacket(payload)

	if got := binary.LittleEndian.Uint16(packet[9:11]); got != 2 {
		t.Fatalf("chunk count = %d, want 2", got)
	}

	second := lyChunkSize
	if got := binary.LittleEndian.Uint16(packet[second+6 : second+8]); got != 0 {
		t.Fatalf("empty last chunk data len = %d, want 0", got)
	}
}

func TestBuildLYPacketIntoReusesBufferAndClearsPadding(t *testing.T) {
	reuse := make([]byte, 0, 8*lyChunkSize)
	first := buildLYPacketInto(make([]byte, lyChunkDataSize*5), reuse)
	for i := range first {
		first[i] = 0xff
	}
	payload := []byte{1, 2, 3}
	second := buildLYPacketInto(payload, first)
	if &second[0] != &first[0] {
		t.Fatal("buildLYPacketInto allocated despite sufficient capacity")
	}
	if got := second[lyChunkHeaderSize : lyChunkHeaderSize+len(payload)]; !bytes.Equal(got, payload) {
		t.Fatalf("payload = %v, want %v", got, payload)
	}
	for i, value := range second[lyChunkHeaderSize+len(payload):] {
		if value != 0 {
			t.Fatalf("stale byte at %d = %x", lyChunkHeaderSize+len(payload)+i, value)
		}
	}
}

func TestBuildLYPacketIntoSteadyStateAllocations(t *testing.T) {
	payload := make([]byte, 64*1024)
	reuse := buildLYPacketInto(payload, nil)
	allocations := testing.AllocsPerRun(100, func() {
		reuse = buildLYPacketInto(payload, reuse)
	})
	if allocations != 0 {
		t.Fatalf("steady-state allocations = %.2f, want 0", allocations)
	}
}

func TestRotateRGBA90Clockwise(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 3))
	setTestPixel(src, 0, 0, color.RGBA{R: 10, A: 255})
	setTestPixel(src, 1, 0, color.RGBA{R: 20, A: 255})
	setTestPixel(src, 0, 1, color.RGBA{R: 30, A: 255})
	setTestPixel(src, 1, 1, color.RGBA{R: 40, A: 255})
	setTestPixel(src, 0, 2, color.RGBA{R: 50, A: 255})
	setTestPixel(src, 1, 2, color.RGBA{R: 60, A: 255})

	got := rotateRGBA(src, 90)
	if got.Bounds().Dx() != 3 || got.Bounds().Dy() != 2 {
		t.Fatalf("rotated size = %dx%d, want 3x2", got.Bounds().Dx(), got.Bounds().Dy())
	}

	wantRed := [][]uint8{
		{50, 30, 10},
		{60, 40, 20},
	}
	for y, row := range wantRed {
		for x, want := range row {
			if got := got.RGBAAt(x, y).R; got != want {
				t.Fatalf("pixel (%d,%d) red = %d, want %d", x, y, got, want)
			}
		}
	}
}

func TestEncodeRGB888JPEGOrientation90OutputsNativeLandscape(t *testing.T) {
	logicalW, logicalH := 462, 1920
	buffer := make([]byte, logicalW*logicalH*3)
	payload, err := encodeRGB888JPEG(buffer, logicalW, logicalH, 1920, 462, 90, 95, "stdlib")
	if err != nil {
		t.Fatalf("encodeRGB888JPEG() error = %v", err)
	}

	cfg, err := jpeg.DecodeConfig(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("DecodeConfig() error = %v", err)
	}
	if cfg.Width != 1920 || cfg.Height != 462 {
		t.Fatalf("JPEG size = %dx%d, want 1920x462", cfg.Width, cfg.Height)
	}
}

func TestEncodeRGB888JPEGOrientation270OutputsNativeLandscape(t *testing.T) {
	logicalW, logicalH := 462, 1920
	buffer := make([]byte, logicalW*logicalH*3)
	payload, err := encodeRGB888JPEG(buffer, logicalW, logicalH, 1920, 462, 270, 95, "stdlib")
	if err != nil {
		t.Fatalf("encodeRGB888JPEG() error = %v", err)
	}

	cfg, err := jpeg.DecodeConfig(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("DecodeConfig() error = %v", err)
	}
	if cfg.Width != 1920 || cfg.Height != 462 {
		t.Fatalf("JPEG size = %dx%d, want 1920x462", cfg.Width, cfg.Height)
	}
}

func setTestPixel(img *image.RGBA, x, y int, c color.RGBA) {
	offset := y*img.Stride + x*4
	img.Pix[offset] = c.R
	img.Pix[offset+1] = c.G
	img.Pix[offset+2] = c.B
	img.Pix[offset+3] = c.A
}
