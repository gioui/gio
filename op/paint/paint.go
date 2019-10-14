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
	size    image.Point
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
	default:
		sz := src.Bounds().Size()
		// Copy the image into a GPU friendly format.
		dst := image.NewRGBA(image.Rectangle{
			Max: sz,
		})
		draw.Draw(dst, src.Bounds(), src, image.Point{}, draw.Src)
		return ImageOp{
			src:  dst,
			size: sz,
		}
	}
}

func (i ImageOp) Size() image.Point {
	return i.size
}

func (i ImageOp) Add(o *op.Ops) {
	if i.uniform {
		ColorOp{
			Color: i.color,
		}.Add(o)
		return
	}
	data := make([]byte, opconst.TypeImageLen)
	data[0] = byte(opconst.TypeImage)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(i.size.X))
	bo.PutUint32(data[5:], uint32(i.size.Y))
	o.Write(data, i.src)
}

func (c ColorOp) Add(o *op.Ops) {
	data := make([]byte, opconst.TypeColorLen)
	data[0] = byte(opconst.TypeColor)
	data[1] = c.Color.R
	data[2] = c.Color.G
	data[3] = c.Color.B
	data[4] = c.Color.A
	o.Write(data)
}

func (d PaintOp) Add(o *op.Ops) {
	data := make([]byte, opconst.TypePaintLen)
	data[0] = byte(opconst.TypePaint)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], math.Float32bits(d.Rect.Min.X))
	bo.PutUint32(data[5:], math.Float32bits(d.Rect.Min.Y))
	bo.PutUint32(data[9:], math.Float32bits(d.Rect.Max.X))
	bo.PutUint32(data[13:], math.Float32bits(d.Rect.Max.Y))
	o.Write(data)
}

// RectClip returns a ClipOp corresponding to a pixel aligned
// rectangular area.
func RectClip(r image.Rectangle) ClipOp {
	return ClipOp{bounds: toRectF(r)}
}

func toRectF(r image.Rectangle) f32.Rectangle {
	return f32.Rectangle{
		Min: f32.Point{X: float32(r.Min.X), Y: float32(r.Min.Y)},
		Max: f32.Point{X: float32(r.Max.X), Y: float32(r.Max.Y)},
	}
}
