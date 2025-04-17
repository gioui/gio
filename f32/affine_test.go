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

func eqaff(x, y Affine2D) bool {
	tol := 1e-5
	return math.Abs(float64(x.a-y.a)) < tol &&
		math.Abs(float64(x.b-y.b)) < tol &&
		math.Abs(float64(x.c-y.c)) < tol &&
		math.Abs(float64(x.d-y.d)) < tol &&
		math.Abs(float64(x.e-y.e)) < tol &&
		math.Abs(float64(x.f-y.f)) < tol
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

func TestString(t *testing.T) {
	tests := []struct {
		in  Affine2D
		exp string
	}{
		{
			in:  NewAffine2D(9, 11, 13, 17, 19, 23),
			exp: "[[9 11 13] [17 19 23]]",
		}, {
			in:  NewAffine2D(29, 31, 37, 43, 47, 53),
			exp: "[[29 31 37] [43 47 53]]",
		}, {
			in:  NewAffine2D(29.142342, 31.4123412, 37.53152, 43.51324213, 47.123412, 53.14312342),
			exp: "[[29.1423 31.4123 37.5315] [43.5132 47.1234 53.1431]]",
		},
	}
	for _, test := range tests {
		if test.in.String() != test.exp {
			t.Errorf("string mismatch: have %q, want %q", test.in.String(), test.exp)
		}
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

func TestPrimes(t *testing.T) {
	xa := NewAffine2D(9, 11, 13, 17, 19, 23)
	xb := NewAffine2D(29, 31, 37, 43, 47, 53)

	pa := Point{X: 2, Y: 3}
	pb := Point{X: 5, Y: 7}

	for _, test := range []struct {
		x   Affine2D
		p   Point
		exp Point
	}{
		{x: xa, p: pa, exp: Pt(64, 114)},
		{x: xa, p: pb, exp: Pt(135, 241)},
		{x: xb, p: pa, exp: Pt(188, 280)},
		{x: xb, p: pb, exp: Pt(399, 597)},
	} {
		got := test.x.Transform(test.p)
		if !eq(got, test.exp) {
			t.Errorf("%v.Transform(%v): have %v, want %v", test.x, test.p, got, test.exp)
		}
	}

	for _, test := range []struct {
		x   Affine2D
		exp Affine2D
	}{
		{x: xa, exp: NewAffine2D(-1.1875, 0.6875, -0.375, 1.0625, -0.5625, -0.875)},
		{x: xb, exp: NewAffine2D(1.5666667, -1.0333333, -3.2000008, -1.4333333, 1-0.03333336, 1.7999992)},
	} {
		got := test.x.Invert()
		if !eqaff(got, test.exp) {
			t.Errorf("%v.Invert(): have %v, want %v", test.x, got, test.exp)
		}
	}

	got := xa.Mul(xb)
	exp := NewAffine2D(734, 796, 929, 1310, 1420, 1659)
	if !eqaff(got, exp) {
		t.Errorf("%v.Mul(%v): have %v, want %v", xa, xb, got, exp)
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

func BenchmarkTransformOffset(b *testing.B) {
	p := Point{X: 1, Y: 2}
	o := Point{X: 0.5, Y: 0.5}
	aff := Affine2D{}.Offset(o)

	for i := 0; i < b.N; i++ {
		p = aff.Transform(p)
	}
	_ = p
}

func BenchmarkTransformScale(b *testing.B) {
	p := Point{X: 1, Y: 2}
	s := Point{X: 0.5, Y: 0.5}
	aff := Affine2D{}.Scale(Point{}, s)
	for i := 0; i < b.N; i++ {
		p = aff.Transform(p)
	}
	_ = p
}

func BenchmarkTransformRotate(b *testing.B) {
	p := Point{X: 1, Y: 2}
	a := float32(math.Pi / 2)
	aff := Affine2D{}.Rotate(Point{}, a)
	for i := 0; i < b.N; i++ {
		p = aff.Transform(p)
	}
	_ = p
}

func BenchmarkTransformTranslateMultiply(b *testing.B) {
	a := Affine2D{}.Offset(Point{X: 1, Y: 1}).Rotate(Point{}, math.Pi/3)
	t := Affine2D{}.Offset(Point{X: 0.5, Y: 0.5})

	for i := 0; i < b.N; i++ {
		a = a.Mul(t)
	}
}

func BenchmarkTransformScaleMultiply(b *testing.B) {
	a := Affine2D{}.Offset(Point{X: 1, Y: 1}).Rotate(Point{}, math.Pi/3)
	t := Affine2D{}.Offset(Point{X: 0.5, Y: 0.5}).Scale(Point{}, Point{X: 0.4, Y: -0.5})

	for i := 0; i < b.N; i++ {
		a = a.Mul(t)
	}
}

func BenchmarkTransformMultiply(b *testing.B) {
	a := Affine2D{}.Offset(Point{X: 1, Y: 1}).Rotate(Point{}, math.Pi/3)
	t := Affine2D{}.Offset(Point{X: 0.5, Y: 0.5}).Rotate(Point{}, math.Pi/7)

	for i := 0; i < b.N; i++ {
		a = a.Mul(t)
	}
}
