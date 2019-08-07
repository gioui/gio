// SPDX-License-Identifier: Unlicense OR MIT

package paint

import (
	"encoding/binary"
	"image"
	"image/color"
	"math"

	"gioui.org/ui"
	"gioui.org/ui/f32"
	"gioui.org/ui/internal/opconst"
)

type ImageOp struct {
	Src  image.Image
	Rect image.Rectangle
}

type ColorOp struct {
	Color color.RGBA
}

type PaintOp struct {
	Rect f32.Rectangle
}

func (i ImageOp) Add(o *ui.Ops) {
	data := make([]byte, opconst.TypeImageLen)
	data[0] = byte(opconst.TypeImage)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(i.Rect.Min.X))
	bo.PutUint32(data[5:], uint32(i.Rect.Min.Y))
	bo.PutUint32(data[9:], uint32(i.Rect.Max.X))
	bo.PutUint32(data[13:], uint32(i.Rect.Max.Y))
	o.Write(data, i.Src)
}

func (c ColorOp) Add(o *ui.Ops) {
	data := make([]byte, opconst.TypeColorLen)
	data[0] = byte(opconst.TypeColor)
	data[1] = c.Color.R
	data[2] = c.Color.G
	data[3] = c.Color.B
	data[4] = c.Color.A
	o.Write(data)
}

func (d PaintOp) Add(o *ui.Ops) {
	data := make([]byte, opconst.TypePaintLen)
	data[0] = byte(opconst.TypePaint)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], math.Float32bits(d.Rect.Min.X))
	bo.PutUint32(data[5:], math.Float32bits(d.Rect.Min.Y))
	bo.PutUint32(data[9:], math.Float32bits(d.Rect.Max.X))
	bo.PutUint32(data[13:], math.Float32bits(d.Rect.Max.Y))
	o.Write(data)
}

// RectClip returns a ClipOp op corresponding to
// a pixel aligned rectangular area.
func RectClip(r image.Rectangle) ClipOp {
	return ClipOp{bounds: toRectF(r)}
}

func toRectF(r image.Rectangle) f32.Rectangle {
	return f32.Rectangle{
		Min: f32.Point{X: float32(r.Min.X), Y: float32(r.Min.Y)},
		Max: f32.Point{X: float32(r.Max.X), Y: float32(r.Max.Y)},
	}
}
