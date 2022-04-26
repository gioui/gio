package clip_test

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

func TestZeroEllipse(t *testing.T) {
	p := image.Pt(1.0, 2.0)
	e := clip.Ellipse{Min: p, Max: p}
	ops := new(op.Ops)
	paint.FillShape(ops, color.NRGBA{R: 255, A: 255}, e.Op(ops))
}
