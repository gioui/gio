// SPDX-License-Identifier: Unlicense OR MIT

package stroke

import (
	"strconv"
	"testing"

	"gioui.org/internal/f32"
)

func TestNormPt(t *testing.T) {
	type scenario struct {
		l           float32
		ptIn, ptOut f32.Point
	}

	scenarios := []scenario{
		// l>0 & X
		{l: +20, ptIn: f32.Point{X: +30, Y: 0}, ptOut: f32.Point{X: +20, Y: 0}},
		{l: +20, ptIn: f32.Point{X: +20, Y: 0}, ptOut: f32.Point{X: +20, Y: 0}},
		{l: +20, ptIn: f32.Point{X: +10, Y: 0}, ptOut: f32.Point{X: +20, Y: 0}},
		{l: +20, ptIn: f32.Point{X: -10, Y: 0}, ptOut: f32.Point{X: -20, Y: 0}},
		{l: +20, ptIn: f32.Point{X: -20, Y: 0}, ptOut: f32.Point{X: -20, Y: 0}},
		{l: +20, ptIn: f32.Point{X: -30, Y: 0}, ptOut: f32.Point{X: -20, Y: 0}},

		// l<0 & X
		{l: -20, ptIn: f32.Point{X: +30, Y: 0}, ptOut: f32.Point{X: -20, Y: 0}},
		{l: -20, ptIn: f32.Point{X: +20, Y: 0}, ptOut: f32.Point{X: -20, Y: 0}},
		{l: -20, ptIn: f32.Point{X: +10, Y: 0}, ptOut: f32.Point{X: -20, Y: 0}},
		{l: -20, ptIn: f32.Point{X: -10, Y: 0}, ptOut: f32.Point{X: +20, Y: 0}},
		{l: -20, ptIn: f32.Point{X: -20, Y: 0}, ptOut: f32.Point{X: +20, Y: 0}},
		{l: -20, ptIn: f32.Point{X: -30, Y: 0}, ptOut: f32.Point{X: +20, Y: 0}},

		// l>0 & Y
		{l: +20, ptIn: f32.Point{X: 0, Y: +30}, ptOut: f32.Point{X: 0, Y: +20}},
		{l: +20, ptIn: f32.Point{X: 0, Y: +20}, ptOut: f32.Point{X: 0, Y: +20}},
		{l: +20, ptIn: f32.Point{X: 0, Y: +10}, ptOut: f32.Point{X: 0, Y: +20}},
		{l: +20, ptIn: f32.Point{X: 0, Y: -10}, ptOut: f32.Point{X: 0, Y: -20}},
		{l: +20, ptIn: f32.Point{X: 0, Y: -20}, ptOut: f32.Point{X: 0, Y: -20}},
		{l: +20, ptIn: f32.Point{X: 0, Y: -30}, ptOut: f32.Point{X: 0, Y: -20}},

		// l<0 & Y
		{l: -20, ptIn: f32.Point{X: 0, Y: +30}, ptOut: f32.Point{X: 0, Y: -20}},
		{l: -20, ptIn: f32.Point{X: 0, Y: +20}, ptOut: f32.Point{X: 0, Y: -20}},
		{l: -20, ptIn: f32.Point{X: 0, Y: +10}, ptOut: f32.Point{X: 0, Y: -20}},
		{l: -20, ptIn: f32.Point{X: 0, Y: -10}, ptOut: f32.Point{X: 0, Y: +20}},
		{l: -20, ptIn: f32.Point{X: 0, Y: -20}, ptOut: f32.Point{X: 0, Y: +20}},
		{l: -20, ptIn: f32.Point{X: 0, Y: -30}, ptOut: f32.Point{X: 0, Y: +20}},

		// l>0 && X=Y
		{l: +20, ptIn: f32.Point{X: +90, Y: +90}, ptOut: f32.Point{X: +14.142137, Y: +14.142137}},
		{l: +20, ptIn: f32.Point{X: +30, Y: +30}, ptOut: f32.Point{X: +14.142136, Y: +14.142136}},
		{l: +20, ptIn: f32.Point{X: +20, Y: +20}, ptOut: f32.Point{X: +14.142136, Y: +14.142136}},
		{l: +20, ptIn: f32.Point{X: +10, Y: +10}, ptOut: f32.Point{X: +14.142136, Y: +14.142136}},
		{l: +20, ptIn: f32.Point{X: -10, Y: -10}, ptOut: f32.Point{X: -14.142136, Y: -14.142136}},
		{l: +20, ptIn: f32.Point{X: -20, Y: -20}, ptOut: f32.Point{X: -14.142136, Y: -14.142136}},
		{l: +20, ptIn: f32.Point{X: -30, Y: -30}, ptOut: f32.Point{X: -14.142136, Y: -14.142136}},
		{l: +20, ptIn: f32.Point{X: -90, Y: -90}, ptOut: f32.Point{X: -14.142137, Y: -14.142137}},

		// l>0 && X=-Y
		{l: +20, ptIn: f32.Point{X: +90, Y: -90}, ptOut: f32.Point{X: +14.142137, Y: -14.142137}},
		{l: +20, ptIn: f32.Point{X: +30, Y: -30}, ptOut: f32.Point{X: +14.142136, Y: -14.142136}},
		{l: +20, ptIn: f32.Point{X: +20, Y: -20}, ptOut: f32.Point{X: +14.142136, Y: -14.142136}},
		{l: +20, ptIn: f32.Point{X: +10, Y: -10}, ptOut: f32.Point{X: +14.142136, Y: -14.142136}},
		{l: +20, ptIn: f32.Point{X: -10, Y: +10}, ptOut: f32.Point{X: -14.142136, Y: +14.142136}},
		{l: +20, ptIn: f32.Point{X: -20, Y: +20}, ptOut: f32.Point{X: -14.142136, Y: +14.142136}},
		{l: +20, ptIn: f32.Point{X: -30, Y: +30}, ptOut: f32.Point{X: -14.142136, Y: +14.142136}},
		{l: +20, ptIn: f32.Point{X: -90, Y: +90}, ptOut: f32.Point{X: -14.142137, Y: +14.142137}},

		// l<0 && X=Y
		{l: -20, ptIn: f32.Point{X: +90, Y: +90}, ptOut: f32.Point{X: -14.142137, Y: -14.142137}},
		{l: -20, ptIn: f32.Point{X: +30, Y: +30}, ptOut: f32.Point{X: -14.142136, Y: -14.142136}},
		{l: -20, ptIn: f32.Point{X: +20, Y: +20}, ptOut: f32.Point{X: -14.142136, Y: -14.142136}},
		{l: -20, ptIn: f32.Point{X: +10, Y: +10}, ptOut: f32.Point{X: -14.142136, Y: -14.142136}},
		{l: -20, ptIn: f32.Point{X: -10, Y: -10}, ptOut: f32.Point{X: +14.142136, Y: +14.142136}},
		{l: -20, ptIn: f32.Point{X: -20, Y: -20}, ptOut: f32.Point{X: +14.142136, Y: +14.142136}},
		{l: -20, ptIn: f32.Point{X: -30, Y: -30}, ptOut: f32.Point{X: +14.142136, Y: +14.142136}},
		{l: -20, ptIn: f32.Point{X: -90, Y: -90}, ptOut: f32.Point{X: +14.142137, Y: +14.142137}},

		// l<0 && X=-Y
		{l: -20, ptIn: f32.Point{X: +90, Y: -90}, ptOut: f32.Point{X: -14.142137, Y: +14.142137}},
		{l: -20, ptIn: f32.Point{X: +30, Y: -30}, ptOut: f32.Point{X: -14.142136, Y: +14.142136}},
		{l: -20, ptIn: f32.Point{X: +20, Y: -20}, ptOut: f32.Point{X: -14.142136, Y: +14.142136}},
		{l: -20, ptIn: f32.Point{X: +10, Y: -10}, ptOut: f32.Point{X: -14.142136, Y: +14.142136}},
		{l: -20, ptIn: f32.Point{X: -10, Y: +10}, ptOut: f32.Point{X: +14.142136, Y: -14.142136}},
		{l: -20, ptIn: f32.Point{X: -20, Y: +20}, ptOut: f32.Point{X: +14.142136, Y: -14.142136}},
		{l: -20, ptIn: f32.Point{X: -30, Y: +30}, ptOut: f32.Point{X: +14.142136, Y: -14.142136}},
		{l: -20, ptIn: f32.Point{X: -90, Y: +90}, ptOut: f32.Point{X: +14.142137, Y: -14.142137}},
	}

	for i, s := range scenarios {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			actual := normPt(s.ptIn, s.l)
			if actual != s.ptOut {
				t.Errorf("in: %v*%v, expected: %v, actual: %v", s.l, s.ptIn, s.ptOut, actual)
			}
		})

	}
}

func BenchmarkSplitCubic(b *testing.B) {
	type scenario struct {
		segments               int
		from, ctrl0, ctrl1, to f32.Point
	}

	scenarios := []scenario{
		{
			segments: 4,
			from:     f32.Pt(0, 0),
			ctrl0:    f32.Pt(10, 10),
			ctrl1:    f32.Pt(10, 10),
			to:       f32.Pt(20, 0),
		},
		{
			segments: 8,
			from:     f32.Pt(-145.90305, 703.21277),
			ctrl0:    f32.Pt(-940.20215, 606.05994),
			ctrl1:    f32.Pt(74.58341, 405.815),
			to:       f32.Pt(104.35474, -241.543),
		},
		{
			segments: 16,
			from:     f32.Pt(770.35626, 639.77765),
			ctrl0:    f32.Pt(735.57135, 545.07837),
			ctrl1:    f32.Pt(286.7138, 853.7052),
			to:       f32.Pt(286.7138, 890.5413),
		},
		{
			segments: 33,
			from:     f32.Pt(0, 0),
			ctrl0:    f32.Pt(0, 0),
			ctrl1:    f32.Pt(100, 100),
			to:       f32.Pt(100, 100),
		},
	}

	for _, s := range scenarios {
		s := s
		b.Run(strconv.Itoa(s.segments), func(b *testing.B) {
			from, ctrl0, ctrl1, to := s.from, s.ctrl0, s.ctrl1, s.to
			quads := make([]QuadSegment, s.segments)
			b.ResetTimer()
			for b.Loop() {
				quads = SplitCubic(from, ctrl0, ctrl1, to, quads[:0])
			}
			if len(quads) != s.segments {
				// this is just for checking that we are benchmarking similar splits
				// when splitting algorithm splits differently, then it's fine to adjust the
				// parameters to give appropriate number of segments.
				b.Fatalf("expected %d but got %d", s.segments, len(quads))
			}
		})
	}
}
