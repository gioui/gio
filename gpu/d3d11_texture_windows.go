// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"image"
	"image/color"

	"gioui.org/op/paint"
)

// A D3D11Texture is a valid paint source: Gio samples it where it is, without
// copying it to system memory.
var _ paint.Texture = (*D3D11Texture)(nil)

// GPUTexture returns the resource Gio should sample.
func (t *D3D11Texture) GPUTexture() any { return t.t }

// ColorModel implements image.Image. The colour model is never used: the
// pixels of a texture are not readable from the CPU.
func (t *D3D11Texture) ColorModel() color.Model { return color.NRGBAModel }

// At implements image.Image. The pixels of a texture are not readable from the
// CPU, so this exists only to satisfy the interface and returns transparent
// black. Nothing in Gio calls it for a texture backed image.
func (t *D3D11Texture) At(x, y int) color.Color { return color.NRGBA{} }

var _ image.Image = (*D3D11Texture)(nil)
