// SPDX-License-Identifier: Unlicense OR MIT

package path

import (
	"unsafe"
)

// The vertex data suitable for passing to vertex programs.
type Vertex struct {
	CornerX, CornerY int16
	MaxY             float32
	FromX, FromY     float32
	CtrlX, CtrlY     float32
	ToX, ToY         float32
}

const VertStride = 7*4 + 2*2

func init() {
	// Check that struct vertex has the expected size and
	// that it contains no padding.
	if unsafe.Sizeof(*(*Vertex)(nil)) != VertStride {
		panic("unexpected struct size")
	}
}
