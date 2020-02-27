// SPDX-License-Identifier: Unlicense OR MIT

package path

import (
	"unsafe"
)

// The vertex data suitable for passing to vertex programs.
type Vertex struct {
	// Corner encodes the corner: +0.5 for south, +.25 for east.
	Corner       float32
	MaxY         float32
	FromX, FromY float32
	CtrlX, CtrlY float32
	ToX, ToY     float32
}

const VertStride = 7*4 + 2*2

func init() {
	// Check that struct vertex has the expected size and
	// that it contains no padding.
	if unsafe.Sizeof(*(*Vertex)(nil)) != VertStride {
		panic("unexpected struct size")
	}
}
