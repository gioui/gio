// SPDX-License-Identifier: Unlicense OR MIT

package rendertest

import (
	"image"
	"image/color"
	"math"
	"testing"

	"golang.org/x/image/colornames"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

func TestPaintRect(t *testing.T) {
	run(t, func(o *op.Ops) {
		paint.FillShape(o, red, clip.Rect(image.Rect(0, 0, 50, 50)).Op())
	}, func(r result) {
		r.expect(0, 0, colornames.Red)
		r.expect(49, 0, colornames.Red)
		r.expect(50, 0, transparent)
		r.expect(10, 50, transparent)
	})
}

func TestPaintClippedRect(t *testing.T) {
	run(t, func(o *op.Ops) {
		defer clip.RRect{Rect: f32.Rect(25, 25, 60, 60)}.Push(o).Pop()
		paint.FillShape(o, red, clip.Rect(image.Rect(0, 0, 50, 50)).Op())
	}, func(r result) {
		r.expect(0, 0, transparent)
		r.expect(24, 35, transparent)
		r.expect(25, 35, colornames.Red)
		r.expect(50, 0, transparent)
		r.expect(10, 50, transparent)
	})
}

func TestPaintClippedCircle(t *testing.T) {
	run(t, func(o *op.Ops) {
		r := float32(10)
		defer clip.RRect{Rect: f32.Rect(20, 20, 40, 40), SE: r, SW: r, NW: r, NE: r}.Push(o).Pop()
		defer clip.Rect(image.Rect(0, 0, 30, 50)).Push(o).Pop()
		paint.Fill(o, red)
	}, func(r result) {
		r.expect(21, 21, transparent)
		r.expect(25, 30, colornames.Red)
		r.expect(31, 30, transparent)
	})
}

func TestPaintArc(t *testing.T) {
	run(t, func(o *op.Ops) {
		p := new(clip.Path)
		p.Begin(o)
		p.Move(f32.Pt(0, 20))
		p.Line(f32.Pt(10, 0))
		p.Arc(f32.Pt(10, 0), f32.Pt(40, 0), math.Pi)
		p.Line(f32.Pt(30, 0))
		p.Line(f32.Pt(0, 25))
		p.Arc(f32.Pt(-10, 5), f32.Pt(10, 15), -math.Pi)
		p.Line(f32.Pt(0, 25))
		p.Arc(f32.Pt(10, 10), f32.Pt(10, 10), 2*math.Pi)
		p.Line(f32.Pt(-10, 0))
		p.Arc(f32.Pt(-10, 0), f32.Pt(-40, 0), -math.Pi)
		p.Line(f32.Pt(-10, 0))
		p.Line(f32.Pt(0, -10))
		p.Arc(f32.Pt(-10, -20), f32.Pt(10, -5), math.Pi)
		p.Line(f32.Pt(0, -10))
		p.Line(f32.Pt(-50, 0))
		p.Close()
		defer clip.Outline{
			Path: p.End(),
		}.Op().Push(o).Pop()

		paint.FillShape(o, red, clip.Rect(image.Rect(0, 0, 128, 128)).Op())
	}, func(r result) {
		r.expect(0, 0, transparent)
		r.expect(0, 25, colornames.Red)
		r.expect(0, 15, transparent)
	})
}

func TestPaintAbsolute(t *testing.T) {
	run(t, func(o *op.Ops) {
		p := new(clip.Path)
		p.Begin(o)
		p.Move(f32.Pt(100, 100)) // offset the initial pen position to test "MoveTo"

		p.MoveTo(f32.Pt(20, 20))
		p.LineTo(f32.Pt(80, 20))
		p.QuadTo(f32.Pt(80, 80), f32.Pt(20, 80))
		p.Close()
		defer clip.Outline{
			Path: p.End(),
		}.Op().Push(o).Pop()

		paint.FillShape(o, red, clip.Rect(image.Rect(0, 0, 128, 128)).Op())
	}, func(r result) {
		r.expect(0, 0, transparent)
		r.expect(30, 30, colornames.Red)
		r.expect(79, 79, transparent)
		r.expect(90, 90, transparent)
	})
}

func TestPaintTexture(t *testing.T) {
	run(t, func(o *op.Ops) {
		squares.Add(o)
		defer scale(80.0/512, 80.0/512).Push(o).Pop()
		paint.PaintOp{}.Add(o)
	}, func(r result) {
		r.expect(0, 0, colornames.Blue)
		r.expect(79, 10, colornames.Green)
		r.expect(80, 0, transparent)
		r.expect(10, 80, transparent)
	})
}

func TestTexturedStrokeClipped(t *testing.T) {
	run(t, func(o *op.Ops) {
		smallSquares.Add(o)
		defer op.Offset(f32.Pt(50, 50)).Push(o).Pop()
		defer clip.Stroke{
			Path:  clip.RRect{Rect: f32.Rect(0, 0, 30, 30)}.Path(o),
			Width: 10,
		}.Op().Push(o).Pop()
		defer clip.RRect{Rect: f32.Rect(-30, -30, 60, 60)}.Push(o).Pop()
		defer op.Offset(f32.Pt(-10, -10)).Push(o).Pop()
		paint.PaintOp{}.Add(o)
	}, func(r result) {
	})
}

func TestTexturedStroke(t *testing.T) {
	run(t, func(o *op.Ops) {
		smallSquares.Add(o)
		defer op.Offset(f32.Pt(50, 50)).Push(o).Pop()
		defer clip.Stroke{
			Path:  clip.RRect{Rect: f32.Rect(0, 0, 30, 30)}.Path(o),
			Width: 10,
		}.Op().Push(o).Pop()
		defer op.Offset(f32.Pt(-10, -10)).Push(o).Pop()
		paint.PaintOp{}.Add(o)
	}, func(r result) {
	})
}

func TestPaintClippedTexture(t *testing.T) {
	run(t, func(o *op.Ops) {
		squares.Add(o)
		defer clip.RRect{Rect: f32.Rect(0, 0, 40, 40)}.Push(o).Pop()
		defer scale(80.0/512, 80.0/512).Push(o).Pop()
		paint.PaintOp{}.Add(o)
	}, func(r result) {
		r.expect(40, 40, transparent)
		r.expect(25, 35, colornames.Blue)
	})
}

func TestStrokedPathZeroWidth(t *testing.T) {
	run(t, func(o *op.Ops) {
		{
			p := new(clip.Path)
			p.Begin(o)
			p.Move(f32.Pt(10, 50))
			p.Line(f32.Pt(30, 0))
			cl := clip.Stroke{
				Path: p.End(),
			}.Op().Push(o) // width=0, disable stroke

			paint.Fill(o, red)
			cl.Pop()
		}

	}, func(r result) {
		r.expect(0, 0, transparent)
		r.expect(10, 50, colornames.Black)
		r.expect(30, 50, colornames.Black)
		r.expect(65, 50, transparent)
	})
}

func TestStrokedPathCoincidentControlPoint(t *testing.T) {
	run(t, func(o *op.Ops) {
		p := new(clip.Path)
		p.Begin(o)
		p.MoveTo(f32.Pt(70, 20))
		p.CubeTo(f32.Pt(70, 20), f32.Pt(70, 110), f32.Pt(120, 120))
		p.LineTo(f32.Pt(20, 120))
		p.LineTo(f32.Pt(70, 20))
		cl := clip.Stroke{
			Path:  p.End(),
			Width: 20,
		}.Op().Push(o)

		paint.Fill(o, black)
		cl.Pop()
	}, func(r result) {
		r.expect(0, 0, transparent)
		r.expect(70, 20, colornames.Black)
		r.expect(70, 90, transparent)
	})
}

func TestPathReuse(t *testing.T) {
	run(t, func(o *op.Ops) {
		var path clip.Path
		path.Begin(o)
		path.MoveTo(f32.Pt(60, 10))
		path.LineTo(f32.Pt(110, 75))
		path.LineTo(f32.Pt(10, 75))
		path.Close()
		spec := path.End()

		outline := clip.Outline{Path: spec}.Op().Push(o)
		paint.Fill(o, color.NRGBA{R: 0xFF, A: 0xFF})
		outline.Pop()

		stroke := clip.Stroke{Path: spec, Width: 3}.Op().Push(o)
		paint.Fill(o, color.NRGBA{B: 0xFF, A: 0xFF})
		stroke.Pop()
	}, func(r result) {
	})
}

func TestPathInterleave(t *testing.T) {
	t.Run("interleave op in clip.Path", func(t *testing.T) {
		defer func() {
			if err := recover(); err == nil {
				t.Error("expected panic did not occur")
			}
		}()
		ops := new(op.Ops)
		var path clip.Path
		path.Begin(ops)
		path.LineTo(f32.Point{X: 123, Y: 456})
		paint.ColorOp{}.Add(ops)
		path.End()
	})
	t.Run("use ops after clip.Path", func(t *testing.T) {
		ops := new(op.Ops)
		var path clip.Path
		path.Begin(ops)
		path.LineTo(f32.Point{X: 123, Y: 456})
		path.End()
		paint.ColorOp{}.Add(ops)
	})
}
