//go:build !linux || !cgo

package nativerender

func blendRGBA(dst, src []byte) {
	for offset := 0; offset+3 < len(dst) && offset+3 < len(src); offset += 4 {
		alpha := uint16(src[offset+3])
		if alpha == 0 {
			continue
		}
		if alpha == 255 {
			copy(dst[offset:offset+4], src[offset:offset+4])
			continue
		}
		inverse := 255 - alpha
		dst[offset] = saturatedByte(uint16(src[offset]) + (uint16(dst[offset])*inverse+127)/255)
		dst[offset+1] = saturatedByte(uint16(src[offset+1]) + (uint16(dst[offset+1])*inverse+127)/255)
		dst[offset+2] = saturatedByte(uint16(src[offset+2]) + (uint16(dst[offset+2])*inverse+127)/255)
		dst[offset+3] = 255
	}
}

func saturatedByte(value uint16) uint8 {
	if value > 255 {
		return 255
	}
	return uint8(value)
}
