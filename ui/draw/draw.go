// SPDX-License-Identifier: Unlicense OR MIT

package draw

import (
	"image"
	"math"

	"gioui.org/ui"
	"gioui.org/ui/f32"
	"gioui.org/ui/internal/path"
)

type OpImage struct {
	Rect    f32.Rectangle
	Src     image.Image
	SrcRect image.Rectangle
}

func (OpImage) ImplementsOp() {}

// ClipRect returns a special case of OpClip
// that clips to a pixel aligned rectangular area.
func ClipRect(r image.Rectangle, op ui.Op) OpClip {
	return OpClip{
		Path: &Path{
			data: &path.Path{
				Bounds: toRectF(r),
			},
		},
		Op: op,
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
