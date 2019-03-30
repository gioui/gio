// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"image"
)

// genAreaLUT generates the lookup table conpatible with the stencilFSrc
// fragment shaders. The table contains the area of a pixel square above
// a line. The square has area 1 and is centered in (0, 0).
// The y-axis intersection of the line in [-8;+8] is specified by the
// first coordinate.
// The slope of the line [0;16] is specified by the second coordinate.
func genAreaLUT(width, height int) *image.Gray {
	lut := image.NewGray(image.Rectangle{Max: image.Point{X: width, Y: height}})
	for v := 0; v < height; v++ {
		a := float32(v) * 16 / float32(height)
		for u := 0; u < width; u++ {
			var area float32
			switch u {
			case 0:
				area = 1.0
			case width - 1:
				area = 0.0
			default:
				b := (float32(u) - float32(width)/2) / 16
				// f(x) = ax+b.
				area = computeLineArea(a, b)
			}
			lut.Pix[v*height+u] = uint8(area*255 + 0.5)
		}
	}
	return lut
}

func computeLineArea(a, b float32) float32 {
	// Compute intersections with the square edges.
	// Right and left.
	ry := a*+0.5 + b
	ly := a*-0.5 + b
	// Top and bottom.
	tx := (+0.5 - b) / a
	bx := (-0.5 - b) / a
	// The line will intersect zero or two edges.
	if ry <= -0.5 {
		// Line is below the square.
		return 1.0
	}
	if ly >= 0.5 {
		// Line is above the square.
		return 0.0
	}
	// The slope is positive, so there are only 4 possible
	// pairs of edges: (bottom, right), (left, right),
	// (bottom, top), (left, top).
	if ry <= 0.5 {
		// Intersection with right edge.
		if ly <= -0.5 {
			// (bottom, right).
			return 1.0 - (0.5-bx)*(ry-(-0.5))/2
		} else {
			// (left, right).
			return 1.0*(0.5-ry) + 1.0*(ry-ly)/2
		}
	} else {
		// Intersection with top edge.
		if ly <= -0.5 {
			// (bottom, top).
			return (bx-(-0.5))*1.0 + (tx-bx)*1.0/2
		} else {
			// (left, top).
			return (tx - (-0.5)) * (0.5 - ly) / 2
		}
	}
}
