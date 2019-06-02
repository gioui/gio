package ui

import (
	"encoding/binary"

	"gioui.org/ui/internal/ops"
)

// Ops holds a list of serialized Ops.
type Ops struct {
	// Stack of block start indices.
	stack []pc
	ops   opsData
}

type opsData struct {
	// Serialized ops.
	data []byte
	// Op references.
	refs []interface{}
}

type OpsReader struct {
	pc    pc
	stack []block
	ops   opsData
}

type block struct {
	ops   opsData
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

type OpPush struct{}

type OpPop struct{}

type OpBlock struct {
	ops *Ops
	pc  pc
}

type opBlockDef struct {
	endpc pc
}

func (p OpPush) Add(o *Ops) {
	o.Write([]byte{byte(ops.TypePush)}, nil)
}

func (p OpPop) Add(o *Ops) {
	o.Write([]byte{byte(ops.TypePop)}, nil)
}

// Begin a block of ops.
func (o *Ops) Begin() {
	o.stack = append(o.stack, o.ops.pc())
	// Make room for a block definition. Filled out in End.
	o.Write(make([]byte, ops.TypeBlockDefLen), nil)
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
	pc := o.ops.pc()
	// Write the block header reserved in Begin.
	data := o.ops.data[start.data : start.data+ops.TypeBlockDefLen]
	data[0] = byte(ops.TypeBlockDef)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(pc.data))
	bo.PutUint32(data[5:], uint32(pc.refs))
	return OpBlock{ops: o, pc: start}
}

// Reset clears the Ops.
func (o *Ops) Reset() {
	o.stack = o.stack[:0]
	o.ops.reset()
}

func (d *opsData) reset() {
	d.data = d.data[:0]
	d.refs = d.refs[:0]
}

func (d *opsData) write(op []byte, refs []interface{}) {
	d.data = append(d.data, op...)
	d.refs = append(d.refs, refs...)
}

func (o *Ops) Write(op []byte, refs []interface{}) {
	o.ops.write(op, refs)
}

func (d *opsData) pc() pc {
	return pc{data: len(d.data), refs: len(d.refs)}
}

func (b *OpBlock) decode(data []byte, refs []interface{}) {
	if ops.OpType(data[0]) != ops.TypeBlock {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	dataIdx := int(bo.Uint32(data[1:]))
	refsIdx := int(bo.Uint32(data[5:]))
	*b = OpBlock{
		ops: refs[0].(*Ops),
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
	o.Write(data, []interface{}{b.ops})
}

// Reset start reading from the op list.
func (r *OpsReader) Reset(ops *Ops) {
	r.ops = ops.ops
	r.stack = r.stack[:0]
	r.pc = pc{}
}

func (r *OpsReader) Decode() ([]byte, []interface{}, bool) {
	for {
		if len(r.stack) > 0 {
			b := r.stack[len(r.stack)-1]
			if r.pc == b.endPC {
				r.ops = b.ops
				r.pc = b.retPC
				r.stack = r.stack[:len(r.stack)-1]
				continue
			}
		}
		if r.pc.data == len(r.ops.data) {
			return nil, nil, false
		}
		t := ops.OpType(r.ops.data[r.pc.data])
		n := typeLengths[t-ops.FirstOpIndex]
		nrefs := refLengths[t-ops.FirstOpIndex]
		data := r.ops.data[r.pc.data : r.pc.data+n]
		refs := r.ops.refs[r.pc.refs : r.pc.refs+nrefs]
		switch t {
		case ops.TypeBlock:
			var op OpBlock
			op.decode(data, refs)
			blockOps := op.ops.ops
			if ops.OpType(blockOps.data[op.pc.data]) != ops.TypeBlockDef {
				panic("invalid block reference")
			}
			var opDef opBlockDef
			opDef.decode(blockOps.data[op.pc.data : op.pc.data+ops.TypeBlockDefLen])
			retPC := r.pc
			retPC.data += n
			retPC.refs += nrefs
			r.stack = append(r.stack, block{
				ops:   r.ops,
				retPC: retPC,
				endPC: opDef.endpc,
			})
			r.ops = blockOps
			r.pc = op.pc
			r.pc.data += ops.TypeBlockDefLen
			r.pc.refs += ops.TypeBlockDefRefs
			continue
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
