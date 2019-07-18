// SPDX-License-Identifier: Unlicense OR MIT

package draw

import (
	"encoding/binary"
	"image"
	"image/color"
	"math"

	"gioui.org/ui"
	"gioui.org/ui/f32"
	"gioui.org/ui/internal/ops"
)

type ImageOp struct {
	Src  image.Image
	Rect image.Rectangle
}

type ColorOp struct {
	Color color.RGBA
}

type DrawOp struct {
	Rect f32.Rectangle
}

func (i ImageOp) Add(o *ui.Ops) {
	data := make([]byte, ops.TypeImageLen)
	data[0] = byte(ops.TypeImage)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(i.Rect.Min.X))
	bo.PutUint32(data[5:], uint32(i.Rect.Min.Y))
	bo.PutUint32(data[9:], uint32(i.Rect.Max.X))
	bo.PutUint32(data[13:], uint32(i.Rect.Max.Y))
	o.Write(data, i.Src)
}

func (i *ImageOp) Decode(data []byte, refs []interface{}) {
	bo := binary.LittleEndian
	if ops.OpType(data[0]) != ops.TypeImage {
		panic("invalid op")
	}
	sr := image.Rectangle{
		Min: image.Point{
			X: int(bo.Uint32(data[1:])),
			Y: int(bo.Uint32(data[5:])),
		},
		Max: image.Point{
			X: int(bo.Uint32(data[9:])),
			Y: int(bo.Uint32(data[13:])),
		},
	}
	*i = ImageOp{
		Src:  refs[0].(image.Image),
		Rect: sr,
	}
}

func (c ColorOp) Add(o *ui.Ops) {
	data := make([]byte, ops.TypeColorLen)
	data[0] = byte(ops.TypeColor)
	data[1] = c.Color.R
	data[2] = c.Color.G
	data[3] = c.Color.B
	data[4] = c.Color.A
	o.Write(data)
}

func (c *ColorOp) Decode(data []byte, refs []interface{}) {
	if ops.OpType(data[0]) != ops.TypeColor {
		panic("invalid op")
	}
	*c = ColorOp{
		Color: color.RGBA{
			R: data[1],
			G: data[2],
			B: data[3],
			A: data[4],
		},
	}
}

func (d DrawOp) Add(o *ui.Ops) {
	data := make([]byte, ops.TypeDrawLen)
	data[0] = byte(ops.TypeDraw)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], math.Float32bits(d.Rect.Min.X))
	bo.PutUint32(data[5:], math.Float32bits(d.Rect.Min.Y))
	bo.PutUint32(data[9:], math.Float32bits(d.Rect.Max.X))
	bo.PutUint32(data[13:], math.Float32bits(d.Rect.Max.Y))
	o.Write(data)
}

func (d *DrawOp) Decode(data []byte, refs []interface{}) {
	bo := binary.LittleEndian
	if ops.OpType(data[0]) != ops.TypeDraw {
		panic("invalid op")
	}
	r := f32.Rectangle{
		Min: f32.Point{
			X: math.Float32frombits(bo.Uint32(data[1:])),
			Y: math.Float32frombits(bo.Uint32(data[5:])),
		},
		Max: f32.Point{
			X: math.Float32frombits(bo.Uint32(data[9:])),
			Y: math.Float32frombits(bo.Uint32(data[13:])),
		},
	}
	*d = DrawOp{
		Rect: r,
	}
}

// RectClip returns a ClipOp op corresponding to
// a pixel aligned rectangular area.
func RectClip(r image.Rectangle) ClipOp {
	return ClipOp{bounds: toRectF(r)}
}

func itof(i int) float32 {
	switch i {
	case ui.Inf:
		return float32(math.Inf(+1))
	case -ui.Inf:
		return float32(math.Inf(-1))
	default:
		return float32(i)
	}
}

func toRectF(r image.Rectangle) f32.Rectangle {
	return f32.Rectangle{
		Min: f32.Point{X: itof(r.Min.X), Y: itof(r.Min.Y)},
		Max: f32.Point{X: itof(r.Max.X), Y: itof(r.Max.Y)},
	}
}
