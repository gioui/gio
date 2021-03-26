// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import "testing"

func TestRectangleContains(t *testing.T) {
	tests := []struct {
		r1, r2 rectangle
		in     bool
	}{
		{
			rectangle{{15, 1754}, {2147, 1754}, {2147, 1501}, {15, 1501}},
			rectangle{{20, 1517}, {20, 1517}, {20, 1517}, {20, 1517}},
			true,
		},
		{
			rectangle{{0, 1882}, {2156, 1882}, {2156, 0}, {0, 0}},
			rectangle{{0, 1882}, {2156, 1882}, {2156, 0}, {0, 0}},
			true,
		},
		{
			rectangle{{-26, 1893}, {2216, 1893}, {2216, -7}, {-26, -7}},
			rectangle{{0, 1882}, {2156, 1882}, {2156, 0}, {0, 0}},
			false,
		},
		{
			rectangle{{0, 1882}, {2156, 1882}, {2156, 0}, {0, 0}},
			rectangle{{-26, 1893}, {2216, 1893}, {2216, -7}, {-26, -7}},
			true,
		},
	}
	for _, test := range tests {
		got := test.r1.In(test.r2)
		if got != test.in {
			t.Errorf("%v.Contains(%v) = %v, expected %v", test.r1, test.r2, got, test.in)
		}
	}
}
