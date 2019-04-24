package ui

import (
	"encoding/binary"

	"gioui.org/ui/internal/ops"
)

// Ops hold a list of serialized Ops.
type Ops struct {
	// Stack of block start indices.
	stack []int
	// Serialized ops.
	data []byte
	// Op references.
	refs []interface{}
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

func (o *Ops) Refs() []interface{} {
	return o.refs
}

func (o *Ops) Data() []byte {
	return o.data
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

func (OpBlock) ImplementsOp() {}
