package ops

import (
	"encoding/binary"
)

type Reader struct {
	pc    int
	stack []block
	Refs  []interface{}
	data  []byte

	pseudoOp [1]byte
}

type block struct {
	retPC int
	endPC int
}

type OpType byte

const (
	TypeBlockDef OpType = iota
	TypeBlock
	TypeTransform
	TypeLayer
	TypeRedraw
	TypeClip
	TypeImage
	TypeDraw
	TypePointerHandler
	TypeKeyHandler
	TypeHideInput
	TypePush
	TypePop
)

const (
	TypeBlockDefLen       = 1 + 4
	TypeBlockLen          = 1 + 4
	TypeTransformLen      = 1 + 4*2
	TypeLayerLen          = 1
	TypeRedrawLen         = 1 + 8
	TypeClipLen           = 1 + 4
	TypeImageLen          = 1 + 4 + 4*4
	TypeDrawLen           = 1 + 4*4
	TypePointerHandlerLen = 1 + 4 + 4 + 1
	TypeKeyHandlerLen     = 1 + 4 + 1
	TypeHideInputLen      = 1
	TypePushLen           = 1
	TypePopLen            = 1
)

var typeLengths = [...]int{
	TypeBlockDefLen,
	TypeBlockLen,
	TypeTransformLen,
	TypeLayerLen,
	TypeRedrawLen,
	TypeClipLen,
	TypeImageLen,
	TypeDrawLen,
	TypePointerHandlerLen,
	TypeKeyHandlerLen,
	TypeHideInputLen,
	TypePushLen,
	TypePopLen,
}

// Reset start reading from the op list.
func (r *Reader) Reset(data []byte, refs []interface{}) {
	r.Refs = refs
	r.data = data
	r.stack = r.stack[:0]
	r.pc = 0
}

func (r *Reader) Decode() ([]byte, bool) {
	bo := binary.LittleEndian
	for {
		if r.pc == len(r.data) {
			return nil, false
		}
		if len(r.stack) > 0 {
			b := r.stack[len(r.stack)-1]
			if r.pc == b.endPC {
				r.pc = b.retPC
				r.stack = r.stack[:len(r.stack)-1]
				r.pseudoOp[0] = byte(TypePop)
				return r.pseudoOp[:], true
			}
		}
		t := OpType(r.data[r.pc])
		n := typeLengths[t]
		data := r.data[r.pc : r.pc+n]
		switch t {
		case TypeBlock:
			blockIdx := int(bo.Uint32(data[1:]))
			if OpType(r.data[blockIdx]) != TypeBlockDef {
				panic("invalid block reference")
			}
			blockLen := int(bo.Uint32(r.data[blockIdx+1:]))
			r.stack = append(r.stack, block{r.pc + n, blockIdx + blockLen})
			r.pc = blockIdx + TypeBlockDefLen
			r.pseudoOp[0] = byte(TypePush)
			return r.pseudoOp[:], true
		case TypeBlockDef:
			r.pc += int(bo.Uint32(data[1:]))
			continue
		}
		r.pc += n
		return data, true
	}
}
