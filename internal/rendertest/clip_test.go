package rendertest

import (
	"testing"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"golang.org/x/image/colornames"
)

func TestPaintRect(t *testing.T) {
	run(t, func(o *op.Ops) {
		paint.ColorOp{Color: colornames.Red}.Add(o)
		paint.PaintOp{Rect: f32.Rect(0, 0, 50, 50)}.Add(o)
	}, func(r result) {
		r.expect(0, 0, colornames.Red)
		r.expect(49, 0, colornames.Red)
		r.expect(50, 0, colornames.White)
		r.expect(10, 50, colornames.White)
	})
}

func TestPaintClippedRect(t *testing.T) {
	run(t, func(o *op.Ops) {
		paint.ColorOp{Color: colornames.Red}.Add(o)
		clip.Rect{Rect: f32.Rect(25, 25, 60, 60)}.Add(o)
		paint.PaintOp{Rect: f32.Rect(0, 0, 50, 50)}.Add(o)
	}, func(r result) {
		r.expect(0, 0, colornames.White)
		r.expect(24, 35, colornames.White)
		r.expect(25, 35, colornames.Red)
		r.expect(50, 0, colornames.White)
		r.expect(10, 50, colornames.White)
	})
}

func TestPaintClippedCirle(t *testing.T) {
	run(t, func(o *op.Ops) {
		paint.ColorOp{Color: colornames.Red}.Add(o)
		r := float32(10)
		clip.Rect{Rect: f32.Rect(20, 20, 40, 40), SE: r, SW: r, NW: r, NE: r}.Add(o)
		paint.PaintOp{Rect: f32.Rect(0, 0, 30, 50)}.Add(o)
	}, func(r result) {
		r.expect(21, 21, colornames.White)
		r.expect(25, 30, colornames.Red)
		r.expect(31, 30, colornames.White)
	})
}

func TestPaintTexture(t *testing.T) {
	run(t, func(o *op.Ops) {
		squares.Add(o)
		paint.PaintOp{Rect: f32.Rect(0, 0, 80, 80)}.Add(o)
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
		clip.Rect{Rect: f32.Rect(0, 0, 40, 40)}.Add(o)
		paint.PaintOp{Rect: f32.Rect(0, 0, 80, 80)}.Add(o)
	}, func(r result) {
		r.expect(40, 40, colornames.White)
		r.expect(25, 35, colornames.Blue)
	})
}
