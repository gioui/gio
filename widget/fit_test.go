// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/layout"
)

func TestFit(t *testing.T) {
	type test struct {
		Dims   image.Point
		Scale  f32.Point
		Result image.Point
	}

	fittests := [...][]test{
		Unscaled: {
			{
				Dims:   image.Point{0, 0},
				Scale:  f32.Point{X: 1, Y: 1},
				Result: image.Point{X: 0, Y: 0},
			}, {
				Dims:   image.Point{50, 25},
				Scale:  f32.Point{X: 1, Y: 1},
				Result: image.Point{X: 50, Y: 25},
			}, {
				Dims:   image.Point{50, 200},
				Scale:  f32.Point{X: 1, Y: 1},
				Result: image.Point{X: 50, Y: 100},
			}},
		Contain: {
			{
				Dims:   image.Point{50, 25},
				Scale:  f32.Point{X: 2, Y: 2},
				Result: image.Point{X: 100, Y: 50},
			}, {
				Dims:   image.Point{50, 200},
				Scale:  f32.Point{X: 0.5, Y: 0.5},
				Result: image.Point{X: 25, Y: 100},
			}},
		Cover: {
			{
				Dims:   image.Point{50, 25},
				Scale:  f32.Point{X: 4, Y: 4},
				Result: image.Point{X: 100, Y: 100},
			}, {
				Dims:   image.Point{50, 200},
				Scale:  f32.Point{X: 2, Y: 2},
				Result: image.Point{X: 100, Y: 100},
			}},
		ScaleDown: {
			{
				Dims:   image.Point{50, 25},
				Scale:  f32.Point{X: 1, Y: 1},
				Result: image.Point{X: 50, Y: 25},
			}, {
				Dims:   image.Point{50, 200},
				Scale:  f32.Point{X: 0.5, Y: 0.5},
				Result: image.Point{X: 25, Y: 100},
			}},
		Fill: {
			{
				Dims:   image.Point{50, 25},
				Scale:  f32.Point{X: 2, Y: 4},
				Result: image.Point{X: 100, Y: 100},
			}, {
				Dims:   image.Point{50, 200},
				Scale:  f32.Point{X: 2, Y: 0.5},
				Result: image.Point{X: 100, Y: 100},
			}},
	}

	for fit, tests := range fittests {
		fit := Fit(fit)
		for i, test := range tests {
			cs := layout.Constraints{
				Max: image.Point{X: 100, Y: 100},
			}
			result, trans := fit.scale(cs, layout.NW, layout.Dimensions{Size: test.Dims})
			sx, _, _, _, sy, _ := trans.Elems()
			if scale := f32.Pt(sx, sy); scale != test.Scale {
				t.Errorf("got scale %v expected %v", scale, test.Scale)
			}

			if result.Size != test.Result {
				t.Errorf("fit %v, #%v: expected %#v, got %#v", fit, i, test.Result, result.Size)
			}
		}
	}
}
