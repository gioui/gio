package rendertest

import (
	"math"
	"testing"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"golang.org/x/image/colornames"
)

func TestTransformMacro(t *testing.T) {
	// testcase resulting from original bug when rendering layout.Stacked

	// pre-build the text
	c := createText()

	run(t, func(o *op.Ops) {

		// render the first Stacked item
		m1 := op.Record(o)
		dr := f32.Rect(0, 0, 128, 50)
		paint.ColorOp{Color: colornames.Black}.Add(o)
		paint.PaintOp{Rect: dr}.Add(o)
		c1 := m1.Stop()

		// Render the second stacked item
		m2 := op.Record(o)
		paint.ColorOp{Color: colornames.Red}.Add(o)
		// Simulate a draw text call
		stack := op.Push(o)
		op.TransformOp{}.Offset(f32.Pt(0, 10)).Add(o)

		// Actually create the text clip-path
		c.Add(o)

		paint.PaintOp{Rect: f32.Rect(0, 0, 10, 10)}.Add(o)
		stack.Pop()

		c2 := m2.Stop()

		// Call each of them in a transform
		s1 := op.Push(o)
		op.TransformOp{}.Offset(f32.Pt(0, 0)).Add(o)
		c1.Add(o)
		s1.Pop()
		s2 := op.Push(o)
		op.TransformOp{}.Offset(f32.Pt(0, 0)).Add(o)
		c2.Add(o)
		s2.Pop()
	}, func(r result) {
		r.expect(5, 15, colornames.Red)
		r.expect(15, 15, colornames.Black)
		r.expect(11, 51, colornames.White)
	})
}

func TestRepeatedPaintsZ(t *testing.T) {
	run(t, func(o *op.Ops) {
		// Draw a rectangle
		paint.ColorOp{Color: colornames.Black}.Add(o)
		paint.PaintOp{Rect: f32.Rect(0, 0, 128, 50)}.Add(o)

		builder := clip.Path{}
		builder.Begin(o)
		builder.Move(f32.Pt(0, 0))
		builder.Line(f32.Pt(10, 0))
		builder.Line(f32.Pt(0, 10))
		builder.Line(f32.Pt(-10, 0))
		builder.Line(f32.Pt(0, -10))
		builder.End().Add(o)
		paint.ColorOp{Color: colornames.Red}.Add(o)
		paint.PaintOp{Rect: f32.Rect(0, 0, 10, 10)}.Add(o)
	}, func(r result) {
		r.expect(5, 5, colornames.Red)
		r.expect(11, 15, colornames.Black)
		r.expect(11, 51, colornames.White)
	})
}

func TestNoClipFromPaint(t *testing.T) {
	// ensure that a paint operation does not polute the state
	// by leaving any clip paths i place.
	run(t, func(o *op.Ops) {
		a := f32.Affine2D{}.Rotate(f32.Pt(20, 20), math.Pi/4)
		op.Affine(a).Add(o)
		paint.ColorOp{Color: colornames.Red}.Add(o)
		paint.PaintOp{Rect: f32.Rect(10, 10, 30, 30)}.Add(o)
		a = f32.Affine2D{}.Rotate(f32.Pt(20, 20), -math.Pi/4)
		op.Affine(a).Add(o)

		paint.ColorOp{Color: colornames.Black}.Add(o)
		paint.PaintOp{Rect: f32.Rect(0, 0, 50, 50)}.Add(o)
	}, func(r result) {
		r.expect(1, 1, colornames.Black)
		r.expect(20, 20, colornames.Black)
		r.expect(49, 49, colornames.Black)
		r.expect(51, 51, colornames.White)
	})
}

func createText() op.CallOp {
	innerOps := new(op.Ops)
	m := op.Record(innerOps)
	builder := clip.Path{}
	builder.Begin(innerOps)
	builder.Move(f32.Pt(0, 0))
	builder.Line(f32.Pt(10, 0))
	builder.Line(f32.Pt(0, 10))
	builder.Line(f32.Pt(-10, 0))
	builder.Line(f32.Pt(0, -10))
	builder.End().Add(innerOps)
	return m.Stop()
}

func drawChild(ops *op.Ops, text op.CallOp) op.CallOp {
	r1 := op.Record(ops)
	text.Add(ops)
	paint.PaintOp{Rect: f32.Rect(0, 0, 10, 10)}.Add(ops)
	return r1.Stop()
}

func TestReuseStencil(t *testing.T) {
	txt := createText()
	run(t, func(ops *op.Ops) {
		c1 := drawChild(ops, txt)
		c2 := drawChild(ops, txt)

		// lay out the children
		stack1 := op.Push(ops)
		c1.Add(ops)
		stack1.Pop()

		stack2 := op.Push(ops)
		op.TransformOp{}.Offset(f32.Pt(0, 50)).Add(ops)
		c2.Add(ops)
		stack2.Pop()
	}, func(r result) {
		r.expect(5, 5, colornames.Black)
		r.expect(5, 55, colornames.Black)
	})
}
