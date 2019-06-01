package ui

import (
	"encoding/binary"

	"gioui.org/ui/internal/ops"
)

// Ops holds a list of serialized Ops.
type Ops struct {
	// Stack of block start indices.
	stack []int
	// Serialized ops.
	data []byte
	// Op references.
	refs []interface{}
}

type OpsReader struct {
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

var typeLengths = [...]int{
	ops.TypeBlockDefLen,
	ops.TypeBlockLen,
	ops.TypeTransformLen,
	ops.TypeLayerLen,
	ops.TypeRedrawLen,
	ops.TypeClipLen,
	ops.TypeImageLen,
	ops.TypeDrawLen,
	ops.TypeColorLen,
	ops.TypePointerHandlerLen,
	ops.TypeKeyHandlerLen,
	ops.TypeHideInputLen,
	ops.TypePushLen,
	ops.TypePopLen,
}

type OpBlock struct {
	idx int
}

// Begin a block of ops.
func (o *Ops) Begin() {
	o.stack = append(o.stack, o.Size())
	data := make([]byte, ops.TypeBlockDefLen)
	data[0] = byte(ops.TypeBlockDef)
	o.Write(data)
}

// End the most recent block and return
// an op for invoking the completed block.
func (o *Ops) End() OpBlock {
	start := o.stack[len(o.stack)-1]
	o.stack = o.stack[:len(o.stack)-1]
	blockLen := o.Size() - start
	bo := binary.LittleEndian
	bo.PutUint32(o.data[start+1:], uint32(blockLen))
	return OpBlock{start}
}

// Reset clears the Ops.
func (o *Ops) Reset() {
	o.refs = o.refs[:0]
	o.stack = o.stack[:0]
	o.data = o.data[:0]
}

func (o *Ops) Ref(r interface{}) int {
	o.refs = append(o.refs, r)
	return len(o.refs) - 1
}

func (o *Ops) Write(op []byte) {
	o.data = append(o.data, op...)
}

// Size returns the length of the serialized Op data.
func (o *Ops) Size() int {
	return len(o.data)
}

func (b OpBlock) Add(o *Ops) {
	data := make([]byte, ops.TypeBlockLen)
	data[0] = byte(ops.TypeBlock)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(b.idx))
	o.Write(data)
}

// Reset start reading from the op list.
func (r *OpsReader) Reset(ops *Ops) {
	r.Refs = ops.refs
	r.data = ops.data
	r.stack = r.stack[:0]
	r.pc = 0
}

func (r *OpsReader) Decode() ([]byte, bool) {
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
				r.pseudoOp[0] = byte(ops.TypePop)
				return r.pseudoOp[:], true
			}
		}
		t := ops.OpType(r.data[r.pc])
		n := typeLengths[t]
		data := r.data[r.pc : r.pc+n]
		switch t {
		case ops.TypeBlock:
			blockIdx := int(bo.Uint32(data[1:]))
			if ops.OpType(r.data[blockIdx]) != ops.TypeBlockDef {
				panic("invalid block reference")
			}
			blockLen := int(bo.Uint32(r.data[blockIdx+1:]))
			r.stack = append(r.stack, block{r.pc + n, blockIdx + blockLen})
			r.pc = blockIdx + ops.TypeBlockDefLen
			r.pseudoOp[0] = byte(ops.TypePush)
			return r.pseudoOp[:], true
		case ops.TypeBlockDef:
			r.pc += int(bo.Uint32(data[1:]))
			continue
		}
		r.pc += n
		return data, true
	}
}
