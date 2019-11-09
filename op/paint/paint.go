// SPDX-License-Identifier: Unlicense OR MIT

package paint

import (
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"math"

	"gioui.org/f32"
	"gioui.org/internal/opconst"
	"gioui.org/op"
)

// ImageOp sets the material to an image.
type ImageOp struct {
	uniform bool
	color   color.RGBA
	src     *image.RGBA

	// handle is a key to uniquely identify this ImageOp
	// in a map of cached textures.
	handle interface{}
}

// ColorOp sets the material to a constant color.
type ColorOp struct {
	Color color.RGBA
}

// PaintOp draws the current material, respecting the
// clip path and transformation.
type PaintOp struct {
	Rect f32.Rectangle
}

func NewImageOp(src image.Image) ImageOp {
	switch src := src.(type) {
	case *image.Uniform:
		col := color.RGBAModel.Convert(src.C).(color.RGBA)
		return ImageOp{
			uniform: true,
			color:   col,
		}
	case *image.RGBA:
		bounds := src.Bounds()
		if bounds.Min == (image.Point{}) && src.Stride == bounds.Dx()*4 {
			return ImageOp{
				src:    src,
				handle: new(int),
			}
		}
	}

	sz := src.Bounds().Size()
	// Copy the image into a GPU friendly format.
	dst := image.NewRGBA(image.Rectangle{
		Max: sz,
	})
	draw.Draw(dst, src.Bounds(), src, image.Point{}, draw.Src)
	return ImageOp{
		src:    dst,
		handle: new(int),
	}
}

func (i ImageOp) Size() image.Point {
	if i.src == nil {
		return image.Point{}
	}
	return i.src.Bounds().Size()
}

func (i ImageOp) Add(o *op.Ops) {
	if i.uniform {
		ColorOp{
			Color: i.color,
		}.Add(o)
		return
	}
	data := o.Write(opconst.TypeImageLen, i.src, i.handle)
	data[0] = byte(opconst.TypeImage)
}

func (c ColorOp) Add(o *op.Ops) {
	data := o.Write(opconst.TypeColorLen)
	data[0] = byte(opconst.TypeColor)
	data[1] = c.Color.R
	data[2] = c.Color.G
	data[3] = c.Color.B
	data[4] = c.Color.A
}

func (d PaintOp) Add(o *op.Ops) {
	data := o.Write(opconst.TypePaintLen)
	data[0] = byte(opconst.TypePaint)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], math.Float32bits(d.Rect.Min.X))
	bo.PutUint32(data[5:], math.Float32bits(d.Rect.Min.Y))
	bo.PutUint32(data[9:], math.Float32bits(d.Rect.Max.X))
	bo.PutUint32(data[13:], math.Float32bits(d.Rect.Max.Y))
}
