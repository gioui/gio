// SPDX-License-Identifier: Unlicense OR MIT

package f32color

import (
	"image/color"
	"testing"
)

func TestNRGBAToRGBA_PostAlpha_Boundary(t *testing.T) {
	for col := 0; col <= 0xFF; col++ {
		for alpha := 0; alpha <= 0xFF; alpha++ {
			in := color.NRGBA{R: uint8(col), A: uint8(alpha)}
			premul := NRGBAToRGBA_PostAlpha(in)
			if premul.A != uint8(alpha) {
				t.Errorf("%v: got %v expected %v", in, premul.A, alpha)
			}
			if premul.R > premul.A {
				t.Errorf("%v: R=%v > A=%v", in, premul.R, premul.A)
			}
		}
	}
}

func TestNRGBAToLinearRGBA_Boundary(t *testing.T) {
	for col := 0; col <= 0xFF; col++ {
		for alpha := 0; alpha <= 0xFF; alpha++ {
			in := color.NRGBA{R: uint8(col), A: uint8(alpha)}
			premul := NRGBAToLinearRGBA(in)
			if premul.A != uint8(alpha) {
				t.Errorf("%v: got %v expected %v", in, premul.A, alpha)
			}
			if premul.R > premul.A {
				t.Errorf("%v: R=%v > A=%v", in, premul.R, premul.A)
			}
		}
	}
}

func TestLinearToRGBARoundtrip(t *testing.T) {
	for col := 0; col <= 0xFF; col++ {
		for alpha := 0; alpha <= 0xFF; alpha++ {
			want := color.NRGBA{R: uint8(col), A: uint8(alpha)}
			if alpha == 0 {
				want.R = 0
			}
			got := LinearFromSRGB(want).SRGB()
			if want != got {
				t.Errorf("got %v expected %v", got, want)
			}
		}
	}
}
