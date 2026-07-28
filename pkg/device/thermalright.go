package device

import "image"

const (
	thermalrightTrofeoWidth  = 1920
	thermalrightTrofeoHeight = 462
)

// ThermalrightTrofeoProfile implements DeviceProfile for the Thermalright
// Trofeo Vision 9.16 LCD. The device uses the Winbond/LY bulk protocol:
// JPEG frame payloads are split into 512-byte USB chunks by the panel layer.
//
// Known VID/PID:
//   - 0416:5408 Thermalright Trofeo Vision 9.16 LCD
type ThermalrightTrofeoProfile struct{}

func (p *ThermalrightTrofeoProfile) ID() string {
	return "thermalright_trofeo_916"
}

func (p *ThermalrightTrofeoProfile) Name() string {
	return "Thermalright Trofeo Vision 9.16 LCD"
}

func (p *ThermalrightTrofeoProfile) Description() string {
	return "1920x462 USB-C widescreen LCD using LY JPEG bulk protocol"
}

func (p *ThermalrightTrofeoProfile) Matches(vendorID, productID uint16) bool {
	return vendorID == 0x0416 && productID == 0x5408
}

func (p *ThermalrightTrofeoProfile) VendorIDs() []uint16 {
	return []uint16{0x0416}
}

func (p *ThermalrightTrofeoProfile) ProductIDs() []uint16 {
	return []uint16{0x5408}
}

func (p *ThermalrightTrofeoProfile) Width() int {
	return thermalrightTrofeoWidth
}

func (p *ThermalrightTrofeoProfile) Height() int {
	return thermalrightTrofeoHeight
}

func (p *ThermalrightTrofeoProfile) ColorFormat() ColorFormat {
	return RGB888
}

func (p *ThermalrightTrofeoProfile) ByteOrder() ByteOrder {
	return BigEndian
}

func (p *ThermalrightTrofeoProfile) BufferSize() int {
	return p.Width() * p.Height() * p.ColorFormat().BytesPerPixel()
}

func (p *ThermalrightTrofeoProfile) MaxBrightness() int {
	return 0
}

func (p *ThermalrightTrofeoProfile) ProtocolType() ProtocolType {
	return ProtocolLYBulk
}

func (p *ThermalrightTrofeoProfile) BlitCommand(x, y, w, h int, dataLen int) []byte {
	return nil
}

func (p *ThermalrightTrofeoProfile) BacklightCommand(level int) []byte {
	return nil
}

func (p *ThermalrightTrofeoProfile) ParseResponse(data []byte) error {
	return nil
}

func (p *ThermalrightTrofeoProfile) ConvertImage(img image.Image) []byte {
	bounds := img.Bounds()
	width := p.Width()
	height := p.Height()
	if bounds.Dx()*bounds.Dy() == p.Width()*p.Height() {
		width = bounds.Dx()
		height = bounds.Dy()
	}
	buffer := make([]byte, width*height*3)

	if rgba, ok := img.(*image.RGBA); ok && bounds.Dx() == width && bounds.Dy() == height {
		idx := 0
		for y := 0; y < height; y++ {
			src := (bounds.Min.Y+y-rgba.Rect.Min.Y)*rgba.Stride + (bounds.Min.X-rgba.Rect.Min.X)*4
			for x := 0; x < width; x++ {
				buffer[idx] = rgba.Pix[src]
				buffer[idx+1] = rgba.Pix[src+1]
				buffer[idx+2] = rgba.Pix[src+2]
				idx += 3
				src += 4
			}
		}
		return buffer
	}

	idx := 0
	for y := 0; y < height; y++ {
		srcY := bounds.Min.Y + (y * bounds.Dy() / height)
		for x := 0; x < width; x++ {
			srcX := bounds.Min.X + (x * bounds.Dx() / width)

			r, g, b, _ := img.At(srcX, srcY).RGBA()
			buffer[idx] = uint8(r >> 8)
			buffer[idx+1] = uint8(g >> 8)
			buffer[idx+2] = uint8(b >> 8)
			idx += 3
		}
	}

	return buffer
}
