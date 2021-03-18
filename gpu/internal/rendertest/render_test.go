package rendertest

import (
	"image"
	"image/color"
	"math"
	"testing"

	"golang.org/x/image/colornames"

	"gioui.org/f32"
	"gioui.org/internal/f32color"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

func TestTransformMacro(t *testing.T) {
	// testcase resulting from original bug when rendering layout.Stacked

	// Build clip-path.
	c := constSqPath()

	run(t, func(o *op.Ops) {

		// render the first Stacked item
		m1 := op.Record(o)
		dr := image.Rect(0, 0, 128, 50)
		paint.FillShape(o, black, clip.Rect(dr).Op())
		c1 := m1.Stop()

		// Render the second stacked item
		m2 := op.Record(o)
		paint.ColorOp{Color: red}.Add(o)
		// Simulate a draw text call
		stack := op.Save(o)
		op.Offset(f32.Pt(0, 10)).Add(o)

		// Apply the clip-path.
		c.Add(o)

		paint.PaintOp{}.Add(o)
		stack.Load()

		c2 := m2.Stop()

		// Call each of them in a transform
		s1 := op.Save(o)
		op.Offset(f32.Pt(0, 0)).Add(o)
		c1.Add(o)
		s1.Load()
		s2 := op.Save(o)
		op.Offset(f32.Pt(0, 0)).Add(o)
		c2.Add(o)
		s2.Load()
	}, func(r result) {
		r.expect(5, 15, colornames.Red)
		r.expect(15, 15, colornames.Black)
		r.expect(11, 51, colornames.White)
	})
}

func TestRepeatedPaintsZ(t *testing.T) {
	run(t, func(o *op.Ops) {
		// Draw a rectangle
		paint.FillShape(o, black, clip.Rect(image.Rect(0, 0, 128, 50)).Op())

		builder := clip.Path{}
		builder.Begin(o)
		builder.Move(f32.Pt(0, 0))
		builder.Line(f32.Pt(10, 0))
		builder.Line(f32.Pt(0, 10))
		builder.Line(f32.Pt(-10, 0))
		builder.Line(f32.Pt(0, -10))
		p := builder.End()
		clip.Outline{
			Path: p,
		}.Op().Add(o)
		paint.Fill(o, red)
	}, func(r result) {
		r.expect(5, 5, colornames.Red)
		r.expect(11, 15, colornames.Black)
		r.expect(11, 51, colornames.White)
	})
}

func TestNoClipFromPaint(t *testing.T) {
	// ensure that a paint operation does not pollute the state
	// by leaving any clip paths in place.
	run(t, func(o *op.Ops) {
		a := f32.Affine2D{}.Rotate(f32.Pt(20, 20), math.Pi/4)
		op.Affine(a).Add(o)
		paint.FillShape(o, red, clip.Rect(image.Rect(10, 10, 30, 30)).Op())
		a = f32.Affine2D{}.Rotate(f32.Pt(20, 20), -math.Pi/4)
		op.Affine(a).Add(o)

		paint.FillShape(o, black, clip.Rect(image.Rect(0, 0, 50, 50)).Op())
	}, func(r result) {
		r.expect(1, 1, colornames.Black)
		r.expect(20, 20, colornames.Black)
		r.expect(49, 49, colornames.Black)
		r.expect(51, 51, colornames.White)
	})
}

func TestDeferredPaint(t *testing.T) {
	run(t, func(o *op.Ops) {
		state := op.Save(o)
		clip.Rect(image.Rect(0, 0, 80, 80)).Op().Add(o)
		paint.ColorOp{Color: color.NRGBA{A: 0xff, G: 0xff}}.Add(o)
		paint.PaintOp{}.Add(o)

		op.Affine(f32.Affine2D{}.Offset(f32.Pt(20, 20))).Add(o)
		m := op.Record(o)
		clip.Rect(image.Rect(0, 0, 80, 80)).Op().Add(o)
		paint.ColorOp{Color: color.NRGBA{A: 0xff, R: 0xff, G: 0xff}}.Add(o)
		paint.PaintOp{}.Add(o)
		paintMacro := m.Stop()
		op.Defer(o, paintMacro)

		state.Load()
		op.Affine(f32.Affine2D{}.Offset(f32.Pt(10, 10))).Add(o)
		clip.Rect(image.Rect(0, 0, 80, 80)).Op().Add(o)
		paint.ColorOp{Color: color.NRGBA{A: 0xff, B: 0xff}}.Add(o)
		paint.PaintOp{}.Add(o)
	}, func(r result) {
	})
}

func constSqPath() op.CallOp {
	innerOps := new(op.Ops)
	m := op.Record(innerOps)
	builder := clip.Path{}
	builder.Begin(innerOps)
	builder.Move(f32.Pt(0, 0))
	builder.Line(f32.Pt(10, 0))
	builder.Line(f32.Pt(0, 10))
	builder.Line(f32.Pt(-10, 0))
	builder.Line(f32.Pt(0, -10))
	p := builder.End()
	clip.Outline{Path: p}.Op().Add(innerOps)
	return m.Stop()
}

func constSqCirc() op.CallOp {
	innerOps := new(op.Ops)
	m := op.Record(innerOps)
	clip.RRect{Rect: f32.Rect(0, 0, 40, 40),
		NW: 20, NE: 20, SW: 20, SE: 20}.Add(innerOps)
	return m.Stop()
}

func drawChild(ops *op.Ops, text op.CallOp) op.CallOp {
	r1 := op.Record(ops)
	text.Add(ops)
	paint.PaintOp{}.Add(ops)
	return r1.Stop()
}

func TestReuseStencil(t *testing.T) {
	txt := constSqPath()
	run(t, func(ops *op.Ops) {
		c1 := drawChild(ops, txt)
		c2 := drawChild(ops, txt)

		// lay out the children
		stack1 := op.Save(ops)
		c1.Add(ops)
		stack1.Load()

		stack2 := op.Save(ops)
		op.Offset(f32.Pt(0, 50)).Add(ops)
		c2.Add(ops)
		stack2.Load()
	}, func(r result) {
		r.expect(5, 5, colornames.Black)
		r.expect(5, 55, colornames.Black)
	})
}

func TestBuildOffscreen(t *testing.T) {
	// Check that something we in one frame build outside the screen
	// still is rendered correctly if moved into the screen in a later
	// frame.

	txt := constSqCirc()
	draw := func(off float32, o *op.Ops) {
		s := op.Save(o)
		op.Offset(f32.Pt(0, off)).Add(o)
		txt.Add(o)
		paint.PaintOp{}.Add(o)
		s.Load()
	}

	multiRun(t,
		frame(
			func(ops *op.Ops) {
				draw(-100, ops)
			}, func(r result) {
				r.expect(5, 5, colornames.White)
				r.expect(20, 20, colornames.White)
			}),
		frame(
			func(ops *op.Ops) {
				draw(0, ops)
			}, func(r result) {
				r.expect(2, 2, colornames.White)
				r.expect(20, 20, colornames.Black)
				r.expect(38, 38, colornames.White)
			}))
}

func TestNegativeOverlaps(t *testing.T) {
	run(t, func(ops *op.Ops) {
		clip.RRect{Rect: f32.Rect(50, 50, 100, 100)}.Add(ops)
		clip.Rect(image.Rect(0, 120, 100, 122)).Add(ops)
		paint.PaintOp{}.Add(ops)
	}, func(r result) {
		r.expect(60, 60, colornames.White)
		r.expect(60, 110, colornames.White)
		r.expect(60, 120, colornames.White)
		r.expect(60, 122, colornames.White)
	})
}

func TestDepthOverlap(t *testing.T) {
	run(t, func(ops *op.Ops) {
		stack := op.Save(ops)
		paint.FillShape(ops, red, clip.Rect{Max: image.Pt(128, 64)}.Op())
		stack.Load()

		stack = op.Save(ops)
		paint.FillShape(ops, green, clip.Rect{Max: image.Pt(64, 128)}.Op())
		stack.Load()
	}, func(r result) {
		r.expect(96, 32, colornames.Red)
		r.expect(32, 96, colornames.Green)
		r.expect(32, 32, colornames.Green)
	})
}

type Gradient struct {
	From, To color.NRGBA
}

var gradients = []Gradient{
	{From: color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xFF}, To: color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}},
	{From: color.NRGBA{R: 0x19, G: 0xFF, B: 0x19, A: 0xFF}, To: color.NRGBA{R: 0xFF, G: 0x19, B: 0x19, A: 0xFF}},
	{From: color.NRGBA{R: 0xFF, G: 0x19, B: 0x19, A: 0xFF}, To: color.NRGBA{R: 0x19, G: 0x19, B: 0xFF, A: 0xFF}},
	{From: color.NRGBA{R: 0x19, G: 0x19, B: 0xFF, A: 0xFF}, To: color.NRGBA{R: 0x19, G: 0xFF, B: 0x19, A: 0xFF}},
	{From: color.NRGBA{R: 0x19, G: 0xFF, B: 0xFF, A: 0xFF}, To: color.NRGBA{R: 0xFF, G: 0x19, B: 0x19, A: 0xFF}},
	{From: color.NRGBA{R: 0xFF, G: 0xFF, B: 0x19, A: 0xFF}, To: color.NRGBA{R: 0x19, G: 0x19, B: 0xFF, A: 0xFF}},
}

func TestLinearGradient(t *testing.T) {
	t.Skip("linear gradients don't support transformations")

	const gradienth = 8
	// 0.5 offset from ends to ensure that the center of the pixel
	// aligns with gradient from and to colors.
	pixelAligned := f32.Rect(0.5, 0, 127.5, gradienth)
	samples := []int{0, 12, 32, 64, 96, 115, 127}

	run(t, func(ops *op.Ops) {
		gr := f32.Rect(0, 0, 128, gradienth)
		for _, g := range gradients {
			paint.LinearGradientOp{
				Stop1:  f32.Pt(gr.Min.X, gr.Min.Y),
				Color1: g.From,
				Stop2:  f32.Pt(gr.Max.X, gr.Min.Y),
				Color2: g.To,
			}.Add(ops)
			st := op.Save(ops)
			clip.RRect{Rect: gr}.Add(ops)
			op.Affine(f32.Affine2D{}.Offset(pixelAligned.Min)).Add(ops)
			scale(pixelAligned.Dx()/128, 1).Add(ops)
			paint.PaintOp{}.Add(ops)
			st.Load()
			gr = gr.Add(f32.Pt(0, gradienth))
		}
	}, func(r result) {
		gr := pixelAligned
		for _, g := range gradients {
			from := f32color.LinearFromSRGB(g.From)
			to := f32color.LinearFromSRGB(g.To)
			for _, p := range samples {
				exp := lerp(from, to, float32(p)/float32(r.img.Bounds().Dx()-1))
				r.expect(p, int(gr.Min.Y+gradienth/2), f32color.NRGBAToRGBA(exp.SRGB()))
			}
			gr = gr.Add(f32.Pt(0, gradienth))
		}
	})
}

func TestLinearGradientAngled(t *testing.T) {
	run(t, func(ops *op.Ops) {
		paint.LinearGradientOp{
			Stop1:  f32.Pt(64, 64),
			Color1: black,
			Stop2:  f32.Pt(0, 0),
			Color2: red,
		}.Add(ops)
		st := op.Save(ops)
		clip.Rect(image.Rect(0, 0, 64, 64)).Add(ops)
		paint.PaintOp{}.Add(ops)
		st.Load()

		paint.LinearGradientOp{
			Stop1:  f32.Pt(64, 64),
			Color1: white,
			Stop2:  f32.Pt(128, 0),
			Color2: green,
		}.Add(ops)
		st = op.Save(ops)
		clip.Rect(image.Rect(64, 0, 128, 64)).Add(ops)
		paint.PaintOp{}.Add(ops)
		st.Load()

		paint.LinearGradientOp{
			Stop1:  f32.Pt(64, 64),
			Color1: black,
			Stop2:  f32.Pt(128, 128),
			Color2: blue,
		}.Add(ops)
		st = op.Save(ops)
		clip.Rect(image.Rect(64, 64, 128, 128)).Add(ops)
		paint.PaintOp{}.Add(ops)
		st.Load()

		paint.LinearGradientOp{
			Stop1:  f32.Pt(64, 64),
			Color1: white,
			Stop2:  f32.Pt(0, 128),
			Color2: magenta,
		}.Add(ops)
		st = op.Save(ops)
		clip.Rect(image.Rect(0, 64, 64, 128)).Add(ops)
		paint.PaintOp{}.Add(ops)
		st.Load()
	}, func(r result) {})
}

func TestRadialGradient(t *testing.T) {
	run(t, func(ops *op.Ops) {
		paint.RadialGradientOp{
			Stop1:  f32.Pt(64, 64),
			Color1: red,
			Stop2:  f32.Pt(64, 0),
			Color2: black,
		}.Add(ops)
		paint.PaintOp{}.Add(ops)
	}, func(r result) {})
}

func TestRadialGradientOffset(t *testing.T) {
	run(t, func(ops *op.Ops) {
		paint.RadialGradientOp{
			Stop1:   f32.Pt(64, 64),
			Color1:  blue,
			Stop2:   f32.Pt(64, 0),
			Color2:  black,
			Offset1: 0.5,
		}.Add(ops)
		paint.PaintOp{}.Add(ops)
	}, func(r result) {})
}

func TestEllipseGradient(t *testing.T) {
	run(t, func(ops *op.Ops) {
		paint.RadialGradientOp{
			Stop1:   f32.Pt(64, 64),
			Color1:  red,
			Stop2:   f32.Pt(64, 0),
			Color2:  black,
			RadiusY: 32,
		}.Add(ops)
		paint.PaintOp{}.Add(ops)
	}, func(r result) {})
}

func TestEllipseGradientAngled(t *testing.T) {
	run(t, func(ops *op.Ops) {
		paint.RadialGradientOp{
			Stop1:   f32.Pt(64, 64),
			Color1:  red,
			Stop2:   f32.Pt(32, 96),
			Color2:  black,
			RadiusY: 32,
		}.Add(ops)
		paint.PaintOp{}.Add(ops)
	}, func(r result) {})
}

// lerp calculates linear interpolation with color b and p.
func lerp(a, b f32color.RGBA, p float32) f32color.RGBA {
	return f32color.RGBA{
		R: a.R*(1-p) + b.R*p,
		G: a.G*(1-p) + b.G*p,
		B: a.B*(1-p) + b.B*p,
		A: a.A*(1-p) + b.A*p,
	}
}
