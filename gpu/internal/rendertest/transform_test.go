// SPDX-License-Identifier: Unlicense OR MIT

package rendertest

import (
	"image"
	"math"
	"testing"

	"golang.org/x/image/colornames"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

func TestPaintOffset(t *testing.T) {
	run(t, func(o *op.Ops) {
		defer op.Offset(image.Pt(10, 20)).Push(o).Pop()
		paint.FillShape(o, red, clip.Rect(image.Rect(0, 0, 50, 50)).Op())
	}, func(r result) {
		r.expect(0, 0, transparent)
		r.expect(59, 30, colornames.Red)
		r.expect(60, 30, transparent)
		r.expect(10, 70, transparent)
	})
}

func TestPaintRotate(t *testing.T) {
	run(t, func(o *op.Ops) {
		a := f32.Affine2D{}.Rotate(f32.Pt(40, 40), -math.Pi/8)
		defer op.Affine(a).Push(o).Pop()
		paint.FillShape(o, red, clip.Rect(image.Rect(20, 20, 60, 60)).Op())
	}, func(r result) {
		r.expect(40, 40, colornames.Red)
		r.expect(50, 19, colornames.Red)
		r.expect(59, 19, transparent)
		r.expect(21, 21, transparent)
	})
}

func TestPaintShear(t *testing.T) {
	run(t, func(o *op.Ops) {
		a := f32.Affine2D{}.Shear(f32.Point{}, math.Pi/4, 0)
		defer op.Affine(a).Push(o).Pop()
		paint.FillShape(o, red, clip.Rect(image.Rect(0, 0, 40, 40)).Op())
	}, func(r result) {
		r.expect(10, 30, transparent)
	})
}

func TestClipPaintOffset(t *testing.T) {
	run(t, func(o *op.Ops) {
		defer clip.RRect{Rect: image.Rect(10, 10, 30, 30)}.Push(o).Pop()
		defer op.Offset(image.Pt(20, 20)).Push(o).Pop()
		paint.FillShape(o, red, clip.Rect(image.Rect(0, 0, 100, 100)).Op())
	}, func(r result) {
		r.expect(0, 0, transparent)
		r.expect(19, 19, transparent)
		r.expect(20, 20, colornames.Red)
		r.expect(30, 30, transparent)
	})
}

func TestClipOffset(t *testing.T) {
	run(t, func(o *op.Ops) {
		defer op.Offset(image.Pt(20, 20)).Push(o).Pop()
		defer clip.RRect{Rect: image.Rect(10, 10, 30, 30)}.Push(o).Pop()
		paint.FillShape(o, red, clip.Rect(image.Rect(0, 0, 100, 100)).Op())
	}, func(r result) {
		r.expect(0, 0, transparent)
		r.expect(29, 29, transparent)
		r.expect(30, 30, colornames.Red)
		r.expect(49, 49, colornames.Red)
		r.expect(50, 50, transparent)
	})
}

func TestClipScale(t *testing.T) {
	run(t, func(o *op.Ops) {
		a := f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(2, 2)).Offset(f32.Pt(10, 10))
		defer op.Affine(a).Push(o).Pop()
		defer clip.RRect{Rect: image.Rect(10, 10, 20, 20)}.Push(o).Pop()
		paint.FillShape(o, red, clip.Rect(image.Rect(0, 0, 1000, 1000)).Op())
	}, func(r result) {
		r.expect(19+10, 19+10, transparent)
		r.expect(20+10, 20+10, colornames.Red)
		r.expect(39+10, 39+10, colornames.Red)
		r.expect(40+10, 40+10, transparent)
	})
}

func TestClipRotate(t *testing.T) {
	run(t, func(o *op.Ops) {
		defer op.Affine(f32.Affine2D{}.Rotate(f32.Pt(40, 40), -math.Pi/4)).Push(o).Pop()
		defer clip.RRect{Rect: image.Rect(30, 30, 50, 50)}.Push(o).Pop()
		paint.FillShape(o, red, clip.Rect(image.Rect(0, 40, 100, 100)).Op())
	}, func(r result) {
		r.expect(39, 39, transparent)
		r.expect(41, 41, colornames.Red)
		r.expect(50, 50, transparent)
	})
}

func TestOffsetTexture(t *testing.T) {
	run(t, func(o *op.Ops) {
		defer op.Offset(image.Pt(15, 15)).Push(o).Pop()
		squares.Add(o)
		defer scale(50.0/512, 50.0/512).Push(o).Pop()
		paint.PaintOp{}.Add(o)
	}, func(r result) {
		r.expect(14, 20, transparent)
		r.expect(66, 20, transparent)
		r.expect(16, 64, colornames.Green)
		r.expect(64, 16, colornames.Green)
	})
}

func TestOffsetScaleTexture(t *testing.T) {
	run(t, func(o *op.Ops) {
		defer op.Offset(image.Pt(15, 15)).Push(o).Pop()
		squares.Add(o)
		defer op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(2, 1))).Push(o).Pop()
		defer scale(50.0/512, 50.0/512).Push(o).Pop()
		paint.PaintOp{}.Add(o)
	}, func(r result) {
		r.expect(114, 64, colornames.Blue)
		r.expect(116, 64, transparent)
	})
}

func TestRotateTexture(t *testing.T) {
	run(t, func(o *op.Ops) {
		squares.Add(o)
		a := f32.Affine2D{}.Offset(f32.Pt(30, 30)).Rotate(f32.Pt(40, 40), math.Pi/4)
		defer op.Affine(a).Push(o).Pop()
		defer scale(20.0/512, 20.0/512).Push(o).Pop()
		paint.PaintOp{}.Add(o)
	}, func(r result) {
		r.expect(40, 40-12, colornames.Blue)
		r.expect(40+12, 40, colornames.Green)
	})
}

func TestRotateClipTexture(t *testing.T) {
	run(t, func(o *op.Ops) {
		squares.Add(o)
		a := f32.Affine2D{}.Rotate(f32.Pt(40, 40), math.Pi/8)
		defer op.Affine(a).Push(o).Pop()
		defer clip.RRect{Rect: image.Rect(30, 30, 50, 50)}.Push(o).Pop()
		defer op.Affine(f32.Affine2D{}.Offset(f32.Pt(10, 10))).Push(o).Pop()
		defer scale(60.0/512, 60.0/512).Push(o).Pop()
		paint.PaintOp{}.Add(o)
	}, func(r result) {
		r.expect(0, 0, transparent)
		r.expect(37, 39, colornames.Green)
		r.expect(36, 39, colornames.Green)
		r.expect(35, 39, colornames.Green)
		r.expect(34, 39, colornames.Green)
		r.expect(33, 39, colornames.Green)
	})
}

func TestComplicatedTransform(t *testing.T) {
	run(t, func(o *op.Ops) {
		squares.Add(o)

		defer clip.RRect{Rect: image.Rect(0, 0, 100, 100), SE: 50, SW: 50, NW: 50, NE: 50}.Push(o).Pop()

		a := f32.Affine2D{}.Shear(f32.Point{}, math.Pi/4, 0)
		defer op.Affine(a).Push(o).Pop()
		defer clip.RRect{Rect: image.Rect(0, 0, 50, 40)}.Push(o).Pop()

		defer scale(50.0/512, 50.0/512).Push(o).Pop()
		paint.PaintOp{}.Add(o)
	}, func(r result) {
		r.expect(20, 5, transparent)
	})
}

func TestTransformOrder(t *testing.T) {
	// check the ordering of operations bot in affine and in gpu stack.
	run(t, func(o *op.Ops) {
		a := f32.Affine2D{}.Offset(f32.Pt(64, 64))
		defer op.Affine(a).Push(o).Pop()

		b := f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(8, 8))
		defer op.Affine(b).Push(o).Pop()

		c := f32.Affine2D{}.Offset(f32.Pt(-10, -10)).Scale(f32.Point{}, f32.Pt(0.5, 0.5))
		defer op.Affine(c).Push(o).Pop()
		paint.FillShape(o, red, clip.Rect(image.Rect(0, 0, 20, 20)).Op())
	}, func(r result) {
		// centered and with radius 40
		r.expect(64-41, 64, transparent)
		r.expect(64-39, 64, colornames.Red)
		r.expect(64+39, 64, colornames.Red)
		r.expect(64+41, 64, transparent)
	})
}
