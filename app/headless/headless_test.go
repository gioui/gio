// SPDX-License-Identifier: Unlicense OR MIT

package headless

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/paint"
)

func TestHeadless(t *testing.T) {
	sz := image.Point{X: 800, Y: 600}
	w, err := NewWindow(sz.X, sz.Y)
	if err != nil {
		t.Skipf("headless windows not supported: %v", err)
	}

	col := color.RGBA{A: 0xff, R: 0xcc, G: 0xcc}
	var ops op.Ops
	paint.ColorOp{Color: col}.Add(&ops)
	// Paint only part of the screen to avoid the glClear optimization.
	paint.PaintOp{Rect: f32.Rectangle{Max: f32.Point{
		X: float32(sz.X) - 100,
		Y: float32(sz.Y) - 100,
	}}}.Add(&ops)
	w.Frame(&ops)

	img, err := w.Screenshot()
	if err != nil {
		t.Fatal(err)
	}
	if isz := img.Bounds().Size(); isz != sz {
		t.Errorf("got %v screenshot, expected %v", isz, sz)
	}
	if got := img.RGBAAt(0, 0); got != col {
		t.Errorf("got color %v, expected %v", got, col)
	}
}
