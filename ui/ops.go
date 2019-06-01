package ui

import (
	"encoding/binary"

	"gioui.org/ui/internal/ops"
)

// Ops holds a list of serialized Ops.
type Ops struct {
	// Stack of block start indices.
	stack []pc
	// Serialized ops.
	data []byte
	// Op references.
	refs []interface{}
}

type OpsReader struct {
	pc    pc
	stack []block
	refs  []interface{}
	data  []byte

	pseudoOp [1]byte
}

type block struct {
	retPC pc
	endPC pc
}

type pc struct {
	data int
	refs int
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

var refLengths = [...]int{
	ops.TypeBlockDefRefs,
	ops.TypeBlockRefs,
	ops.TypeTransformRefs,
	ops.TypeLayerRefs,
	ops.TypeRedrawRefs,
	ops.TypeClipRefs,
	ops.TypeImageRefs,
	ops.TypeDrawRefs,
	ops.TypeColorRefs,
	ops.TypePointerHandlerRefs,
	ops.TypeKeyHandlerRefs,
	ops.TypeHideInputRefs,
	ops.TypePushRefs,
	ops.TypePopRefs,
}

type OpBlock struct {
	pc pc
}

type opBlockDef struct {
	endpc pc
}

// Begin a block of ops.
func (o *Ops) Begin() {
	o.stack = append(o.stack, o.pc())
	// Make room for a block definition. Filled out in End.
	o.data = append(o.data, make([]byte, ops.TypeBlockDefLen)...)
}

func (op *opBlockDef) decode(data []byte) {
	if ops.OpType(data[0]) != ops.TypeBlockDef {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	dataIdx := int(bo.Uint32(data[1:]))
	refsIdx := int(bo.Uint32(data[5:]))
	*op = opBlockDef{
		endpc: pc{
			data: dataIdx,
			refs: refsIdx,
		},
	}
}

// End the most recent block and return
// an op for invoking the completed block.
func (o *Ops) End() OpBlock {
	start := o.stack[len(o.stack)-1]
	o.stack = o.stack[:len(o.stack)-1]
	pc := o.pc()
	// Write the block header reserved in Begin.
	data := o.data[start.data : start.data+ops.TypeBlockDefLen]
	data[0] = byte(ops.TypeBlockDef)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(pc.data))
	bo.PutUint32(data[5:], uint32(pc.refs))
	return OpBlock{start}
}

// Reset clears the Ops.
func (o *Ops) Reset() {
	o.refs = o.refs[:0]
	o.stack = o.stack[:0]
	o.data = o.data[:0]
}

func (o *Ops) Write(op []byte, refs []interface{}) {
	o.data = append(o.data, op...)
	o.refs = append(o.refs, refs...)
}

func (o *Ops) pc() pc {
	return pc{data: len(o.data), refs: len(o.refs)}
}

func (b *OpBlock) decode(data []byte) {
	if ops.OpType(data[0]) != ops.TypeBlock {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	dataIdx := int(bo.Uint32(data[1:]))
	refsIdx := int(bo.Uint32(data[5:]))
	*b = OpBlock{
		pc: pc{
			data: dataIdx,
			refs: refsIdx,
		},
	}
}

func (b OpBlock) Add(o *Ops) {
	data := make([]byte, ops.TypeBlockLen)
	data[0] = byte(ops.TypeBlock)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(b.pc.data))
	bo.PutUint32(data[5:], uint32(b.pc.refs))
	o.Write(data, nil)
}

// Reset start reading from the op list.
func (r *OpsReader) Reset(ops *Ops) {
	r.refs = ops.refs
	r.data = ops.data
	r.stack = r.stack[:0]
	r.pc = pc{}
}

func (r *OpsReader) Decode() ([]byte, []interface{}, bool) {
	for {
		if r.pc.data == len(r.data) {
			return nil, nil, false
		}
		if len(r.stack) > 0 {
			b := r.stack[len(r.stack)-1]
			if r.pc == b.endPC {
				r.pc = b.retPC
				r.stack = r.stack[:len(r.stack)-1]
				r.pseudoOp[0] = byte(ops.TypePop)
				return r.pseudoOp[:], nil, true
			}
		}
		t := ops.OpType(r.data[r.pc.data])
		n := typeLengths[t-ops.FirstOpIndex]
		nrefs := refLengths[t-ops.FirstOpIndex]
		data := r.data[r.pc.data : r.pc.data+n]
		refs := r.refs[r.pc.refs : r.pc.refs+nrefs]
		switch t {
		case ops.TypeBlock:
			var op OpBlock
			op.decode(data)
			if ops.OpType(r.data[op.pc.data]) != ops.TypeBlockDef {
				panic("invalid block reference")
			}
			var opDef opBlockDef
			opDef.decode(r.data[op.pc.data : op.pc.data+ops.TypeBlockDefLen])
			retPC := r.pc
			retPC.data += n
			retPC.refs += nrefs
			r.stack = append(r.stack, block{retPC: retPC, endPC: opDef.endpc})
			r.pc = op.pc
			r.pc.data += ops.TypeBlockDefLen
			r.pc.refs += ops.TypeBlockDefRefs
			r.pseudoOp[0] = byte(ops.TypePush)
			return r.pseudoOp[:], nil, true
		case ops.TypeBlockDef:
			var op opBlockDef
			op.decode(data)
			r.pc = op.endpc
			continue
		}
		r.pc.data += n
		r.pc.refs += nrefs
		return data, refs, true
	}
}
