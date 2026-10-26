// SPDX-License-Identifier: Unlicense OR MIT

//go:build darwin && !ios

package app

import (
	"math"
	"testing"
)

func TestMagnifyScrollDirection(t *testing.T) {
	if got := magnifyScroll(0.1); got >= 0 {
		t.Fatalf("opening pinch must produce negative Scroll.Y (zoom in), got %v", got)
	}
	if got := magnifyScroll(-0.1); got <= 0 {
		t.Fatalf("closing pinch must produce positive Scroll.Y (zoom out), got %v", got)
	}
	if got := magnifyScroll(0); got != 0 {
		t.Fatalf("zero delta must produce zero scroll, got %v", got)
	}
}

func TestMagnifyScrollMatchesWheelZoom(t *testing.T) {
	// wheelZoom: factor = 1.2^(-Scroll.Y/50).  A 10% pinch should be close to
	// the previous gpofd behaviour (factor 1+0.1).
	y := float64(magnifyScroll(0.1))
	factor := math.Pow(1.2, -y/50)
	if math.Abs(factor-1.1) > 0.02 {
		t.Fatalf("10%% pinch factor = %v, want ~1.1", factor)
	}
}
