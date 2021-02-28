// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"bytes"
	"encoding/binary"
	"image"
	"math"
	"testing"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
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
			ops := new(op.Ops)
			gtx := layout.Context{
				Ops: ops,
				Constraints: layout.Constraints{
					Max: image.Point{X: 100, Y: 100},
				},
			}

			result := fit.scale(gtx, layout.NW, layout.Dimensions{Size: test.Dims})

			if test.Scale.X != 1 || test.Scale.Y != 1 {
				opsdata := gtx.Ops.Data()
				scaleX := float32Bytes(test.Scale.X)
				scaleY := float32Bytes(test.Scale.Y)
				if !bytes.Contains(opsdata, scaleX) {
					t.Errorf("did not find scale.X:%v (%x) in ops: %x", test.Scale.X, scaleX, opsdata)
				}
				if !bytes.Contains(opsdata, scaleY) {
					t.Errorf("did not find scale.Y:%v (%x) in ops: %x", test.Scale.Y, scaleY, opsdata)
				}
			}

			if result.Size != test.Result {
				t.Errorf("fit %v, #%v: expected %#v, got %#v", fit, i, test.Result, result.Size)
			}
		}
	}
}

func float32Bytes(v float32) []byte {
	var dst [4]byte
	binary.LittleEndian.PutUint32(dst[:], math.Float32bits(v))
	return dst[:]
}
