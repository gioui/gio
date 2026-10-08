// SPDX-License-Identifier: Unlicense OR MIT

package paint

import "image"

// Texture is an image whose pixels already live in a GPU texture, for example
// the output of a custom rendering pass. Painting a Texture uploads no pixels:
// the blitter samples the texture where it is.
//
// A Texture is only useful with backends that can adopt foreign resources. On
// backends that cannot, painting falls back to reading At, which most
// implementations do not support.
type Texture interface {
	image.Image
	// GPUTexture returns an opaque value identifying the GPU resource
	// backing the image. It must be unique per texture and stable for as
	// long as the texture can be painted.
	GPUTexture() any
}

// NewImageOpFromTexture creates an ImageOp that paints a texture already
// resident on the GPU, without copying it to system memory.
func NewImageOpFromTexture(t Texture) ImageOp {
	return ImageOp{
		ext:    t,
		handle: t,
		Filter: FilterLinear,
	}
}
