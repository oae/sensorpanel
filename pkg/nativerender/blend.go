package nativerender

import (
	"image"
)

// Composite overlays the renderer's RGBA layer onto an opaque background using
// the same premultiplied-alpha semantics as image/draw. A specialized cgo
// implementation is used on Linux.
func (r *Renderer) Composite(dst, overlay *image.RGBA) {
	if dst == nil || overlay == nil || dst.Bounds() != overlay.Bounds() ||
		dst.Bounds().Min != (image.Point{}) {
		return
	}
	blendRGBA(dst.Pix, overlay.Pix)
}
