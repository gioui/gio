// SPDX-License-Identifier: Unlicense OR MIT

package headless

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

func TestHeadless(t *testing.T) {
	w, release := newTestWindow(t)
	defer release()

	sz := w.size
	col := color.RGBA{A: 0xff, R: 0xca, G: 0xfe}
	var ops op.Ops
	paint.ColorOp{Color: col}.Add(&ops)
	// Paint only part of the screen to avoid the glClear optimization.
	paint.FillShape(&ops, col, clip.Rect(image.Rect(0, 0, sz.X-100, sz.Y-100)).Op())
	if err := w.Frame(&ops); err != nil {
		t.Fatal(err)
	}

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

func TestClipping(t *testing.T) {
	w, release := newTestWindow(t)
	defer release()

	col := color.RGBA{A: 0xff, R: 0xca, G: 0xfe}
	col2 := color.RGBA{A: 0xff, R: 0x00, G: 0xfe}
	var ops op.Ops
	paint.ColorOp{Color: col}.Add(&ops)
	clip.RRect{
		Rect: f32.Rectangle{
			Min: f32.Point{X: 50, Y: 50},
			Max: f32.Point{X: 250, Y: 250},
		},
		SE: 75,
	}.Add(&ops)
	paint.PaintOp{}.Add(&ops)
	paint.ColorOp{Color: col2}.Add(&ops)
	clip.RRect{
		Rect: f32.Rectangle{
			Min: f32.Point{X: 100, Y: 100},
			Max: f32.Point{X: 350, Y: 350},
		},
		NW: 75,
	}.Add(&ops)
	paint.PaintOp{}.Add(&ops)
	if err := w.Frame(&ops); err != nil {
		t.Fatal(err)
	}

	img, err := w.Screenshot()
	if err != nil {
		t.Fatal(err)
	}
	if *dumpImages {
		if err := saveImage("clip.png", img); err != nil {
			t.Fatal(err)
		}
	}
	bg := color.RGBA{A: 0xff, R: 0xff, G: 0xff, B: 0xff}
	tests := []struct {
		x, y  int
		color color.RGBA
	}{
		{120, 120, col},
		{130, 130, col2},
		{210, 210, col2},
		{230, 230, bg},
	}
	for _, test := range tests {
		if got := img.RGBAAt(test.x, test.y); got != test.color {
			t.Errorf("(%d,%d): got color %v, expected %v", test.x, test.y, got, test.color)
		}
	}
}

func TestDepth(t *testing.T) {
	w, release := newTestWindow(t)
	defer release()
	var ops op.Ops

	blue := color.RGBA{B: 0xFF, A: 0xFF}
	paint.FillShape(&ops, blue, clip.Rect(image.Rect(0, 0, 50, 100)).Op())
	red := color.RGBA{R: 0xFF, A: 0xFF}
	paint.FillShape(&ops, red, clip.Rect(image.Rect(0, 0, 100, 50)).Op())
	if err := w.Frame(&ops); err != nil {
		t.Fatal(err)
	}

	img, err := w.Screenshot()
	if err != nil {
		t.Fatal(err)
	}
	if *dumpImages {
		if err := saveImage("depth.png", img); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		x, y  int
		color color.RGBA
	}{
		{25, 25, red},
		{75, 25, red},
		{25, 75, blue},
	}
	for _, test := range tests {
		if got := img.RGBAAt(test.x, test.y); got != test.color {
			t.Errorf("(%d,%d): got color %v, expected %v", test.x, test.y, got, test.color)
		}
	}
}

func newTestWindow(t *testing.T) (*Window, func()) {
	t.Helper()
	sz := image.Point{X: 800, Y: 600}
	w, err := NewWindow(sz.X, sz.Y)
	if err != nil {
		t.Skipf("headless windows not supported: %v", err)
	}
	return w, func() {
		w.Release()
	}
}
