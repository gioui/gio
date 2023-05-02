// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"
	"testing"

	"gioui.org/op"
)

func TestStack(t *testing.T) {
	gtx := Context{
		Ops: new(op.Ops),
		Constraints: Constraints{
			Max: image.Pt(100, 100),
		},
	}
	exp := image.Point{X: 60, Y: 70}
	dims := Stack{Alignment: Center}.Layout(gtx,
		Expanded(func(gtx Context) Dimensions {
			return Dimensions{Size: exp}
		}),
		Stacked(func(gtx Context) Dimensions {
			return Dimensions{Size: image.Point{X: 50, Y: 50}}
		}),
	)
	if got := dims.Size; got != exp {
		t.Errorf("Stack ignored Expanded size, got %v expected %v", got, exp)
	}
}

func TestFlex(t *testing.T) {
	gtx := Context{
		Ops: new(op.Ops),
		Constraints: Constraints{
			Min: image.Pt(100, 100),
			Max: image.Pt(100, 100),
		},
	}
	dims := Flex{}.Layout(gtx)
	if got := dims.Size; got != gtx.Constraints.Min {
		t.Errorf("Flex ignored minimum constraints, got %v expected %v", got, gtx.Constraints.Min)
	}
}

func TestDirection(t *testing.T) {
	max := image.Pt(100, 100)
	for _, tc := range []struct {
		dir Direction
		exp image.Point
	}{
		{N, image.Pt(max.X, 0)},
		{S, image.Pt(max.X, 0)},
		{E, image.Pt(0, max.Y)},
		{W, image.Pt(0, max.Y)},
		{NW, image.Pt(0, 0)},
		{NE, image.Pt(0, 0)},
		{SE, image.Pt(0, 0)},
		{SW, image.Pt(0, 0)},
		{Center, image.Pt(0, 0)},
	} {
		t.Run(tc.dir.String(), func(t *testing.T) {
			gtx := Context{
				Ops:         new(op.Ops),
				Constraints: Exact(max),
			}
			var min image.Point
			tc.dir.Layout(gtx, func(gtx Context) Dimensions {
				min = gtx.Constraints.Min
				return Dimensions{}
			})
			if got, exp := min, tc.exp; got != exp {
				t.Errorf("got %v; expected %v", got, exp)
			}
		})
	}
}

func TestConstraints(t *testing.T) {
	type testcase struct {
		name     string
		in       Constraints
		subMax   image.Point
		addMin   image.Point
		expected Constraints
	}
	for _, tc := range []testcase{
		{
			name:     "no-op",
			in:       Constraints{Max: image.Pt(100, 100)},
			expected: Constraints{Max: image.Pt(100, 100)},
		},
		{
			name:     "shrink max",
			in:       Constraints{Max: image.Pt(100, 100)},
			subMax:   image.Pt(25, 25),
			expected: Constraints{Max: image.Pt(75, 75)},
		},
		{
			name:     "shrink max below min",
			in:       Constraints{Max: image.Pt(100, 100), Min: image.Pt(50, 50)},
			subMax:   image.Pt(75, 75),
			expected: Constraints{Max: image.Pt(25, 25), Min: image.Pt(25, 25)},
		},
		{
			name:     "shrink max below zero",
			in:       Constraints{Max: image.Pt(100, 100), Min: image.Pt(50, 50)},
			subMax:   image.Pt(125, 125),
			expected: Constraints{Max: image.Pt(0, 0), Min: image.Pt(0, 0)},
		},
		{
			name:     "enlarge min",
			in:       Constraints{Max: image.Pt(100, 100)},
			addMin:   image.Pt(25, 25),
			expected: Constraints{Max: image.Pt(100, 100), Min: image.Pt(25, 25)},
		},
		{
			name:     "enlarge min beyond max",
			in:       Constraints{Max: image.Pt(100, 100)},
			addMin:   image.Pt(125, 125),
			expected: Constraints{Max: image.Pt(100, 100), Min: image.Pt(100, 100)},
		},
		{
			name:     "decrease min below zero",
			in:       Constraints{Max: image.Pt(100, 100), Min: image.Pt(50, 50)},
			addMin:   image.Pt(-125, -125),
			expected: Constraints{Max: image.Pt(100, 100), Min: image.Pt(0, 0)},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			start := tc.in
			if tc.subMax != (image.Point{}) {
				start = start.SubMax(tc.subMax)
			}
			if tc.addMin != (image.Point{}) {
				start = start.AddMin(tc.addMin)
			}
			if start != tc.expected {
				t.Errorf("expected %#+v, got %#+v", tc.expected, start)
			}
		})
	}
}
