// SPDX-License-Identifier: Unlicense OR MIT

package fling

import "testing"

func TestDecomposeQR(t *testing.T) {
	A := &matrix{
		rows: 3, cols: 3,
		data: []float32{
			12, 6, -4,
			-51, 167, 24,
			4, -68, -41,
		},
	}
	Q, Rt, ok := decomposeQR(A)
	if !ok {
		t.Fatal("decomposeQR failed")
	}
	R := Rt.transpose()
	QR := Q.mul(R)
	if !A.approxEqual(QR) {
		t.Log("A\n", A)
		t.Log("Q\n", Q)
		t.Log("R\n", R)
		t.Log("QR\n", QR)
		t.Fatal("Q*R not approximately equal to A")
	}
}

func TestFit(t *testing.T) {
	X := []float32{-1, 0, 1}
	Y := []float32{2, 0, 2}

	got, ok := polyFit(X, Y)
	if !ok {
		t.Fatal("polyFit failed")
	}
	want := coefficients{0, 0, 2}
	if !got.approxEqual(want) {
		t.Fatalf("polyFit: got %v want %v", got, want)
	}
}
