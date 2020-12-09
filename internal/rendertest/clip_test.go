package rendertest

import (
	"image"
	"math"
	"testing"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"golang.org/x/image/colornames"
)

func TestPaintRect(t *testing.T) {
	run(t, func(o *op.Ops) {
		paint.FillShape(o, red, clip.Rect(image.Rect(0, 0, 50, 50)).Op())
	}, func(r result) {
		r.expect(0, 0, colornames.Red)
		r.expect(49, 0, colornames.Red)
		r.expect(50, 0, colornames.White)
		r.expect(10, 50, colornames.White)
	})
}

func TestPaintClippedRect(t *testing.T) {
	run(t, func(o *op.Ops) {
		clip.RRect{Rect: f32.Rect(25, 25, 60, 60)}.Add(o)
		paint.FillShape(o, red, clip.Rect(image.Rect(0, 0, 50, 50)).Op())
	}, func(r result) {
		r.expect(0, 0, colornames.White)
		r.expect(24, 35, colornames.White)
		r.expect(25, 35, colornames.Red)
		r.expect(50, 0, colornames.White)
		r.expect(10, 50, colornames.White)
	})
}

func TestPaintClippedBorder(t *testing.T) {
	run(t, func(o *op.Ops) {
		var dashes clip.Dash
		dashes.Begin(o)
		dashes.Phase(1)
		dashes.Dash(2)
		dashes.Dash(1)

		clip.Border{
			Rect:   f32.Rect(25, 25, 60, 60),
			Width:  4,
			Dashes: dashes.End(),
		}.Add(o)
		paint.FillShape(o, red, clip.Rect(image.Rect(0, 0, 50, 50)).Op())
	}, func(r result) {
		r.expect(0, 0, colornames.White)
		r.expect(25, 25, colornames.Red)
		r.expect(50, 0, colornames.White)
		r.expect(10, 50, colornames.White)
	})
}

func TestPaintClippedCirle(t *testing.T) {
	run(t, func(o *op.Ops) {
		r := float32(10)
		clip.RRect{Rect: f32.Rect(20, 20, 40, 40), SE: r, SW: r, NW: r, NE: r}.Add(o)
		clip.Rect(image.Rect(0, 0, 30, 50)).Add(o)
		paint.Fill(o, red)
	}, func(r result) {
		r.expect(21, 21, colornames.White)
		r.expect(25, 30, colornames.Red)
		r.expect(31, 30, colornames.White)
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
		clip.Outline{
			Path: p.End(),
		}.Op().Add(o)

		paint.FillShape(o, red, clip.Rect(image.Rect(0, 0, 128, 128)).Op())
	}, func(r result) {
		r.expect(0, 0, colornames.White)
		r.expect(0, 25, colornames.Red)
		r.expect(0, 15, colornames.White)
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
		clip.Outline{
			Path: p.End(),
		}.Op().Add(o)

		paint.FillShape(o, red, clip.Rect(image.Rect(0, 0, 128, 128)).Op())
	}, func(r result) {
		r.expect(0, 0, colornames.White)
		r.expect(30, 30, colornames.Red)
		r.expect(79, 79, colornames.White)
		r.expect(90, 90, colornames.White)
	})
}

func TestPaintTexture(t *testing.T) {
	run(t, func(o *op.Ops) {
		squares.Add(o)
		scale(80.0/512, 80.0/512).Add(o)
		paint.PaintOp{}.Add(o)
	}, func(r result) {
		r.expect(0, 0, colornames.Blue)
		r.expect(79, 10, colornames.Green)
		r.expect(80, 0, colornames.White)
		r.expect(10, 80, colornames.White)
	})
}

func TestPaintClippedTexture(t *testing.T) {
	run(t, func(o *op.Ops) {
		squares.Add(o)
		clip.RRect{Rect: f32.Rect(0, 0, 40, 40)}.Add(o)
		scale(80.0/512, 80.0/512).Add(o)
		paint.PaintOp{}.Add(o)
	}, func(r result) {
		r.expect(40, 40, colornames.White)
		r.expect(25, 35, colornames.Blue)
	})
}

func TestStrokedPathBevelFlat(t *testing.T) {
	run(t, func(o *op.Ops) {
		p := newStrokedPath(o)
		clip.Stroke{
			Path: p,
			Style: clip.StrokeStyle{
				Width: 2.5,
				Cap:   clip.FlatCap,
				Join:  clip.BevelJoin,
			},
		}.Op().Add(o)

		paint.Fill(o, red)
	}, func(r result) {
		r.expect(0, 0, colornames.White)
		r.expect(10, 50, colornames.Red)
	})
}

func TestStrokedPathBevelRound(t *testing.T) {
	run(t, func(o *op.Ops) {
		p := newStrokedPath(o)
		clip.Stroke{
			Path: p,
			Style: clip.StrokeStyle{
				Width: 2.5,
				Cap:   clip.RoundCap,
				Join:  clip.BevelJoin,
			},
		}.Op().Add(o)

		paint.Fill(o, red)
	}, func(r result) {
		r.expect(0, 0, colornames.White)
		r.expect(10, 50, colornames.Red)
	})
}

func TestStrokedPathBevelSquare(t *testing.T) {
	run(t, func(o *op.Ops) {
		p := newStrokedPath(o)
		clip.Stroke{
			Path: p,
			Style: clip.StrokeStyle{
				Width: 2.5,
				Cap:   clip.SquareCap,
				Join:  clip.BevelJoin,
			},
		}.Op().Add(o)

		paint.Fill(o, red)
	}, func(r result) {
		r.expect(0, 0, colornames.White)
		r.expect(10, 50, colornames.Red)
	})
}

func TestStrokedPathRoundRound(t *testing.T) {
	run(t, func(o *op.Ops) {
		p := newStrokedPath(o)
		clip.Stroke{
			Path: p,
			Style: clip.StrokeStyle{
				Width: 2.5,
				Cap:   clip.RoundCap,
				Join:  clip.RoundJoin,
			},
		}.Op().Add(o)

		paint.Fill(o, red)
	}, func(r result) {
		r.expect(0, 0, colornames.White)
		r.expect(10, 50, colornames.Red)
	})
}

func TestStrokedPathFlatMiter(t *testing.T) {
	run(t, func(o *op.Ops) {
		{
			stk := op.Push(o)
			p := newZigZagPath(o)
			clip.Stroke{
				Path: p,
				Style: clip.StrokeStyle{
					Width: 10,
					Cap:   clip.FlatCap,
					Join:  clip.BevelJoin,
					Miter: 5,
				},
			}.Op().Add(o)
			paint.Fill(o, red)
			stk.Pop()
		}
		{
			stk := op.Push(o)
			p := newZigZagPath(o)
			clip.Stroke{
				Path: p,
				Style: clip.StrokeStyle{
					Width: 2,
				},
			}.Op().Add(o)
			paint.Fill(o, black)
			stk.Pop()
		}

	}, func(r result) {
		r.expect(0, 0, colornames.White)
		r.expect(40, 10, colornames.Black)
		r.expect(40, 12, colornames.Red)
	})
}

func TestStrokedPathFlatMiterInf(t *testing.T) {
	run(t, func(o *op.Ops) {
		{
			stk := op.Push(o)
			p := newZigZagPath(o)
			clip.Stroke{
				Path: p,
				Style: clip.StrokeStyle{
					Width: 10,
					Cap:   clip.FlatCap,
					Join:  clip.BevelJoin,
					Miter: float32(math.Inf(+1)),
				},
			}.Op().Add(o)
			paint.Fill(o, red)
			stk.Pop()
		}
		{
			stk := op.Push(o)
			p := newZigZagPath(o)
			clip.Stroke{
				Path: p,
				Style: clip.StrokeStyle{
					Width: 2,
				},
			}.Op().Add(o)
			paint.Fill(o, black)
			stk.Pop()
		}

	}, func(r result) {
		r.expect(0, 0, colornames.White)
		r.expect(40, 10, colornames.Black)
		r.expect(40, 12, colornames.Red)
	})
}

func TestStrokedPathZeroWidth(t *testing.T) {
	run(t, func(o *op.Ops) {
		{
			stk := op.Push(o)
			p := new(clip.Path)
			p.Begin(o)
			p.Move(f32.Pt(10, 50))
			p.Line(f32.Pt(50, 0))
			clip.Stroke{
				Path: p.End(),
				Style: clip.StrokeStyle{
					Width: 2,
				},
			}.Op().Add(o)

			paint.Fill(o, black)
			stk.Pop()
		}

		{
			stk := op.Push(o)
			p := new(clip.Path)
			p.Begin(o)
			p.Move(f32.Pt(10, 50))
			p.Line(f32.Pt(30, 0))
			clip.Stroke{
				Path: p.End(),
			}.Op().Add(o) // width=0, disable stroke

			paint.Fill(o, red)
			stk.Pop()
		}

	}, func(r result) {
		r.expect(0, 0, colornames.White)
		r.expect(10, 50, colornames.Black)
		r.expect(30, 50, colornames.Black)
		r.expect(65, 50, colornames.White)
	})
}

func TestDashedPathFlatCapEllipse(t *testing.T) {
	run(t, func(o *op.Ops) {
		{
			stk := op.Push(o)
			p := newEllipsePath(o)

			var dash clip.Dash
			dash.Begin(o)
			dash.Dash(5)
			dash.Dash(3)

			clip.Stroke{
				Path: p,
				Style: clip.StrokeStyle{
					Width: 10,
					Cap:   clip.FlatCap,
					Join:  clip.BevelJoin,
					Miter: float32(math.Inf(+1)),
				},
				Dashes: dash.End(),
			}.Op().Add(o)

			paint.Fill(
				o,
				red,
			)
			stk.Pop()
		}
		{
			stk := op.Push(o)
			p := newEllipsePath(o)
			clip.Stroke{
				Path: p,
				Style: clip.StrokeStyle{
					Width: 2,
				},
			}.Op().Add(o)

			paint.Fill(
				o,
				black,
			)
			stk.Pop()
		}

	}, func(r result) {
		r.expect(0, 0, colornames.White)
		r.expect(0, 62, colornames.Red)
		r.expect(0, 65, colornames.Black)
	})
}

func TestDashedPathFlatCapZ(t *testing.T) {
	run(t, func(o *op.Ops) {
		{
			stk := op.Push(o)
			p := newZigZagPath(o)
			var dash clip.Dash
			dash.Begin(o)
			dash.Dash(5)
			dash.Dash(3)

			clip.Stroke{
				Path: p,
				Style: clip.StrokeStyle{
					Width: 10,
					Cap:   clip.FlatCap,
					Join:  clip.BevelJoin,
					Miter: float32(math.Inf(+1)),
				},
				Dashes: dash.End(),
			}.Op().Add(o)
			paint.Fill(o, red)
			stk.Pop()
		}

		{
			stk := op.Push(o)
			p := newZigZagPath(o)
			clip.Stroke{
				Path:  p,
				Style: clip.StrokeStyle{Width: 2},
			}.Op().Add(o)
			paint.Fill(o, black)
			stk.Pop()
		}
	}, func(r result) {
		r.expect(0, 0, colornames.White)
		r.expect(40, 10, colornames.Black)
		r.expect(40, 12, colornames.Red)
		r.expect(46, 12, colornames.White)
	})
}

func TestDashedPathFlatCapZNoDash(t *testing.T) {
	run(t, func(o *op.Ops) {
		{
			stk := op.Push(o)
			p := newZigZagPath(o)
			var dash clip.Dash
			dash.Begin(o)
			dash.Phase(1)

			clip.Stroke{
				Path: p,
				Style: clip.StrokeStyle{
					Width: 10,
					Cap:   clip.FlatCap,
					Join:  clip.BevelJoin,
					Miter: float32(math.Inf(+1)),
				},
				Dashes: dash.End(),
			}.Op().Add(o)
			paint.Fill(o, red)
			stk.Pop()
		}
		{
			stk := op.Push(o)
			clip.Stroke{
				Path:  newZigZagPath(o),
				Style: clip.StrokeStyle{Width: 2},
			}.Op().Add(o)
			paint.Fill(o, black)
			stk.Pop()
		}
	}, func(r result) {
		r.expect(0, 0, colornames.White)
		r.expect(40, 10, colornames.Black)
		r.expect(40, 12, colornames.Red)
		r.expect(46, 12, colornames.Red)
	})
}

func TestDashedPathFlatCapZNoPath(t *testing.T) {
	run(t, func(o *op.Ops) {
		{
			stk := op.Push(o)
			var dash clip.Dash
			dash.Begin(o)
			dash.Dash(0)
			clip.Stroke{
				Path: newZigZagPath(o),
				Style: clip.StrokeStyle{
					Width: 10,
					Cap:   clip.FlatCap,
					Join:  clip.BevelJoin,
					Miter: float32(math.Inf(+1)),
				},
				Dashes: dash.End(),
			}.Op().Add(o)
			paint.Fill(o, red)
			stk.Pop()
		}
		{
			stk := op.Push(o)
			p := newZigZagPath(o)
			clip.Stroke{
				Path:  p,
				Style: clip.StrokeStyle{Width: 2},
			}.Op().Add(o)
			paint.Fill(o, black)
			stk.Pop()
		}
	}, func(r result) {
		r.expect(0, 0, colornames.White)
		r.expect(40, 10, colornames.Black)
		r.expect(40, 12, colornames.White)
		r.expect(46, 12, colornames.White)
	})
}

func newStrokedPath(o *op.Ops) clip.PathSpec {
	p := new(clip.Path)
	p.Begin(o)
	p.Move(f32.Pt(10, 50))
	p.Line(f32.Pt(10, 0))
	p.Arc(f32.Pt(10, 0), f32.Pt(20, 0), math.Pi)
	p.Line(f32.Pt(10, 0))
	p.Line(f32.Pt(10, 10))
	p.Arc(f32.Pt(0, 30), f32.Pt(0, 30), 2*math.Pi)
	p.Line(f32.Pt(-20, 0))
	p.Quad(f32.Pt(-10, -10), f32.Pt(-30, 30))
	return p.End()
}

func newZigZagPath(o *op.Ops) clip.PathSpec {
	p := new(clip.Path)
	p.Begin(o)
	p.Move(f32.Pt(40, 10))
	p.Line(f32.Pt(50, 0))
	p.Line(f32.Pt(-50, 50))
	p.Line(f32.Pt(50, 0))
	p.Quad(f32.Pt(-50, 20), f32.Pt(-50, 50))
	p.Line(f32.Pt(50, 0))
	return p.End()
}

func newEllipsePath(o *op.Ops) clip.PathSpec {
	p := new(clip.Path)
	p.Begin(o)
	p.Move(f32.Pt(0, 65))
	p.Line(f32.Pt(20, 0))
	p.Arc(f32.Pt(20, 0), f32.Pt(70, 0), 2*math.Pi)
	return p.End()
}
