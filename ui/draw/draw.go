// SPDX-License-Identifier: Unlicense OR MIT

package draw

import (
	"encoding/binary"
	"image"
	"math"

	"gioui.org/ui"
	"gioui.org/ui/f32"
	"gioui.org/ui/internal/ops"
	"gioui.org/ui/internal/path"
)

type OpImage struct {
	Rect    f32.Rectangle
	Src     image.Image
	SrcRect image.Rectangle
}

func (i OpImage) Add(o *ui.Ops) {
	data := make([]byte, ops.TypeImageLen)
	data[0] = byte(ops.TypeImage)
	bo := binary.LittleEndian
	ref := o.Ref(i.Src)
	bo.PutUint32(data[1:], uint32(ref))
	bo.PutUint32(data[5:], math.Float32bits(i.Rect.Min.X))
	bo.PutUint32(data[9:], math.Float32bits(i.Rect.Min.Y))
	bo.PutUint32(data[13:], math.Float32bits(i.Rect.Max.X))
	bo.PutUint32(data[17:], math.Float32bits(i.Rect.Max.Y))
	bo.PutUint32(data[21:], uint32(i.SrcRect.Min.X))
	bo.PutUint32(data[25:], uint32(i.SrcRect.Min.Y))
	bo.PutUint32(data[29:], uint32(i.SrcRect.Max.X))
	bo.PutUint32(data[33:], uint32(i.SrcRect.Max.Y))
	o.Write(data)
}

func (i *OpImage) Decode(d []byte, refs []interface{}) {
	bo := binary.LittleEndian
	if ops.OpType(d[0]) != ops.TypeImage {
		panic("invalid op")
	}
	ref := int(bo.Uint32(d[1:]))
	r := f32.Rectangle{
		Min: f32.Point{
			X: math.Float32frombits(bo.Uint32(d[5:])),
			Y: math.Float32frombits(bo.Uint32(d[9:])),
		},
		Max: f32.Point{
			X: math.Float32frombits(bo.Uint32(d[13:])),
			Y: math.Float32frombits(bo.Uint32(d[17:])),
		},
	}
	sr := image.Rectangle{
		Min: image.Point{
			X: int(bo.Uint32(d[21:])),
			Y: int(bo.Uint32(d[25:])),
		},
		Max: image.Point{
			X: int(bo.Uint32(d[29:])),
			Y: int(bo.Uint32(d[33:])),
		},
	}
	*i = OpImage{
		Rect:    r,
		Src:     refs[ref].(image.Image),
		SrcRect: sr,
	}
}

// RectPath constructs a path corresponding to
// a pixel aligned rectangular area.
func RectPath(r image.Rectangle) *Path {
	return &Path{
		data: &path.Path{
			Bounds: toRectF(r),
		},
	}
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
