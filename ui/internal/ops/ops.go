// SPDX-License-Identifier: Unlicense OR MIT

package ops

import (
	"encoding/binary"
	"math"

	"gioui.org/ui"
	"gioui.org/ui/f32"
	"gioui.org/ui/internal/opconst"
)

func DecodeTransformOp(d []byte) ui.TransformOp {
	bo := binary.LittleEndian
	if opconst.OpType(d[0]) != opconst.TypeTransform {
		panic("invalid op")
	}
	return ui.TransformOp{}.Offset(f32.Point{
		X: math.Float32frombits(bo.Uint32(d[1:])),
		Y: math.Float32frombits(bo.Uint32(d[5:])),
	})
}
