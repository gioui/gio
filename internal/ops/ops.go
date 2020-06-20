// SPDX-License-Identifier: Unlicense OR MIT

package ops

import (
	"encoding/binary"
	"math"

	"gioui.org/f32"
	"gioui.org/internal/opconst"
	"gioui.org/op"
)

const QuadSize = 4 * 2 * 3

type Quad struct {
	From, Ctrl, To f32.Point
}

func EncodeQuad(d []byte, q Quad) {
	bo := binary.LittleEndian
	bo.PutUint32(d[0:], math.Float32bits(q.From.X))
	bo.PutUint32(d[4:], math.Float32bits(q.From.Y))
	bo.PutUint32(d[8:], math.Float32bits(q.Ctrl.X))
	bo.PutUint32(d[12:], math.Float32bits(q.Ctrl.Y))
	bo.PutUint32(d[16:], math.Float32bits(q.To.X))
	bo.PutUint32(d[20:], math.Float32bits(q.To.Y))
}

func DecodeQuad(d []byte) (q Quad) {
	bo := binary.LittleEndian
	q.From.X = math.Float32frombits(bo.Uint32(d[0:]))
	q.From.Y = math.Float32frombits(bo.Uint32(d[4:]))
	q.Ctrl.X = math.Float32frombits(bo.Uint32(d[8:]))
	q.Ctrl.Y = math.Float32frombits(bo.Uint32(d[12:]))
	q.To.X = math.Float32frombits(bo.Uint32(d[16:]))
	q.To.Y = math.Float32frombits(bo.Uint32(d[20:]))
	return
}

func DecodeTransformOp(d []byte) op.TransformOp {
	bo := binary.LittleEndian
	if opconst.OpType(d[0]) != opconst.TypeTransform {
		panic("invalid op")
	}
	return op.TransformOp{}.Offset(f32.Point{
		X: math.Float32frombits(bo.Uint32(d[1:])),
		Y: math.Float32frombits(bo.Uint32(d[5:])),
	})
}
