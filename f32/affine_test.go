// SPDX-License-Identifier: Unlicense OR MIT

package f32

import (
	"math"
	"testing"
)

func eq(p1, p2 Point) bool {
	tol := 1e-5
	dx, dy := p2.X-p1.X, p2.Y-p1.Y
	return math.Abs(math.Sqrt(float64(dx*dx+dy*dy))) < tol
}

func TestTransformOffset(t *testing.T) {
	p := Point{X: 1, Y: 2}
	o := Point{X: 2, Y: -3}

	r := Affine2D{}.Offset(o).Transform(p)
	if !eq(r, Pt(3, -1)) {
		t.Errorf("offset transformation mismatch: have %v, want {3 -1}", r)
	}
	i := Affine2D{}.Offset(o).Invert().Transform(r)
	if !eq(i, p) {
		t.Errorf("offset transformation inverse mismatch: have %v, want %v", i, p)
	}
}

func TestTransformScale(t *testing.T) {
	p := Point{X: 1, Y: 2}
	s := Point{X: -1, Y: 2}

	r := Affine2D{}.Scale(Point{}, s).Transform(p)
	if !eq(r, Pt(-1, 4)) {
		t.Errorf("scale transformation mismatch: have %v, want {-1 4}", r)
	}
	i := Affine2D{}.Scale(Point{}, s).Invert().Transform(r)
	if !eq(i, p) {
		t.Errorf("scale transformation inverse mismatch: have %v, want %v", i, p)
	}
}

func TestTransformRotate(t *testing.T) {
	p := Point{X: 1, Y: 0}
	a := float32(math.Pi / 2)

	r := Affine2D{}.Rotate(Point{}, a).Transform(p)
	if !eq(r, Pt(0, 1)) {
		t.Errorf("rotate transformation mismatch: have %v, want {0 1}", r)
	}
	i := Affine2D{}.Rotate(Point{}, a).Invert().Transform(r)
	if !eq(i, p) {
		t.Errorf("rotate transformation inverse mismatch: have %v, want %v", i, p)
	}
}

func TestTransformShear(t *testing.T) {
	p := Point{X: 1, Y: 1}

	r := Affine2D{}.Shear(Point{}, math.Pi/4, 0).Transform(p)
	if !eq(r, Pt(2, 1)) {
		t.Errorf("shear transformation mismatch: have %v, want {2 1}", r)
	}
	i := Affine2D{}.Shear(Point{}, math.Pi/4, 0).Invert().Transform(r)
	if !eq(i, p) {
		t.Errorf("shear transformation inverse mismatch: have %v, want %v", i, p)
	}
}

func TestTransformMultiply(t *testing.T) {
	p := Point{X: 1, Y: 2}
	o := Point{X: 2, Y: -3}
	s := Point{X: -1, Y: 2}
	a := float32(-math.Pi / 2)

	r := Affine2D{}.Offset(o).Scale(Point{}, s).Rotate(Point{}, a).Shear(Point{}, math.Pi/4, 0).Transform(p)
	if !eq(r, Pt(1, 3)) {
		t.Errorf("complex transformation mismatch: have %v, want {1 3}", r)
	}
	i := Affine2D{}.Offset(o).Scale(Point{}, s).Rotate(Point{}, a).Shear(Point{}, math.Pi/4, 0).Invert().Transform(r)
	if !eq(i, p) {
		t.Errorf("complex transformation inverse mismatch: have %v, want %v", i, p)
	}
}

func TestTransformScaleAround(t *testing.T) {
	p := Pt(-1, -1)
	target := Pt(-6, -13)
	pt := Affine2D{}.Scale(Pt(4, 5), Pt(2, 3)).Transform(p)
	if !eq(pt, target) {
		t.Log(pt, "!=", target)
		t.Error("Scale not as expected")
	}
}

func TestTransformRotateAround(t *testing.T) {
	p := Pt(-1, -1)
	pt := Affine2D{}.Rotate(Pt(1, 1), -math.Pi/2).Transform(p)
	target := Pt(-1, 3)
	if !eq(pt, target) {
		t.Log(pt, "!=", target)
		t.Error("Rotate not as expected")
	}
}

func TestMulOrder(t *testing.T) {
	A := Affine2D{}.Offset(Pt(100, 100))
	B := Affine2D{}.Scale(Point{}, Pt(2, 2))
	_ = A
	_ = B

	T1 := Affine2D{}.Offset(Pt(100, 100)).Scale(Point{}, Pt(2, 2))
	T2 := B.Mul(A)

	if T1 != T2 {
		t.Log(T1)
		t.Log(T2)
		t.Error("multiplication / transform order not as expected")
	}
}
