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

	inAux  bool
	auxOff int
	auxLen int
}

type opsData struct {
	version int
	// Serialized ops.
	data []byte
	// Op references.
	refs []interface{}
}

// OpsReader parses an ops list. Internal use only.
type OpsReader struct {
	pc    pc
	stack []block
	ops   *opsData
}

// EncodedOp represents an encoded op returned by
// OpsReader. Internal use only.
type EncodedOp struct {
	Key  OpKey
	Data []byte
	Refs []interface{}
}

// OpKey is a unique key for a given op. Internal use only.
type OpKey struct {
	ops     *opsData
	pc      int
	version int
}

type block struct {
	ops   *opsData
	retPC pc
	endPC pc
}

type pc struct {
	data int
	refs int
}

type PushOp struct{}

type PopOp struct{}

type BlockOp struct {
	ops     *opsData
	version int
	pc      pc
}

type opBlockDef struct {
	endpc pc
}

type opAux struct {
	len int
}

func (p PushOp) Add(o *Ops) {
	o.Write([]byte{byte(ops.TypePush)})
}

func (p PopOp) Add(o *Ops) {
	o.Write([]byte{byte(ops.TypePop)})
}

// Begin a block of ops.
func (o *Ops) Begin() {
	o.stack = append(o.stack, o.ops.pc())
	// Make room for a block definition. Filled out in End.
	o.Write(make([]byte, ops.TypeBlockDefLen))
}

func (op *opAux) decode(data []byte) {
	if ops.OpType(data[0]) != ops.TypeAux {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	*op = opAux{
		len: int(bo.Uint32(data[1:])),
	}
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
func (o *Ops) End() BlockOp {
	start := o.stack[len(o.stack)-1]
	o.stack = o.stack[:len(o.stack)-1]
	pc := o.ops.pc()
	// Write the block header reserved in Begin.
	data := o.ops.data[start.data : start.data+ops.TypeBlockDefLen]
	data[0] = byte(ops.TypeBlockDef)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(pc.data))
	bo.PutUint32(data[5:], uint32(pc.refs))
	return BlockOp{ops: &o.ops, pc: start, version: o.ops.version}
}

// Reset the Ops, preparing it for re-use.
func (o *Ops) Reset() {
	o.inAux = false
	o.stack = o.stack[:0]
	o.ops.reset()
}

// Internal use only.
func (o *Ops) Aux() []byte {
	if !o.inAux {
		return nil
	}
	return o.ops.data[o.auxOff+ops.TypeAuxLen : o.auxOff+ops.TypeAuxLen+o.auxLen]
}

func (d *opsData) reset() {
	d.data = d.data[:0]
	d.refs = d.refs[:0]
	d.version++
}

func (d *opsData) write(op []byte, refs ...interface{}) {
	d.data = append(d.data, op...)
	d.refs = append(d.refs, refs...)
}

func (o *Ops) Write(op []byte, refs ...interface{}) {
	t := ops.OpType(op[0])
	if len(refs) != t.NumRefs() {
		panic("invalid ref count")
	}
	switch t {
	case ops.TypeAux:
		// Write only the data.
		op = op[1:]
		if !o.inAux {
			o.inAux = true
			o.auxOff = o.ops.pc().data
			o.auxLen = 0
			header := make([]byte, ops.TypeAuxLen)
			header[0] = byte(ops.TypeAux)
			o.ops.write(header)
		}
		o.auxLen += len(op)
	default:
		if o.inAux {
			o.inAux = false
			bo := binary.LittleEndian
			bo.PutUint32(o.ops.data[o.auxOff+1:], uint32(o.auxLen))
		}
	}
	o.ops.write(op, refs...)
}

func (d *opsData) pc() pc {
	return pc{data: len(d.data), refs: len(d.refs)}
}

func (b *BlockOp) decode(data []byte, refs []interface{}) {
	if ops.OpType(data[0]) != ops.TypeBlock {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	dataIdx := int(bo.Uint32(data[1:]))
	refsIdx := int(bo.Uint32(data[5:]))
	version := int(bo.Uint32(data[9:]))
	*b = BlockOp{
		ops: refs[0].(*opsData),
		pc: pc{
			data: dataIdx,
			refs: refsIdx,
		},
		version: version,
	}
}

func (b BlockOp) Add(o *Ops) {
	data := make([]byte, ops.TypeBlockLen)
	data[0] = byte(ops.TypeBlock)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(b.pc.data))
	bo.PutUint32(data[5:], uint32(b.pc.refs))
	bo.PutUint32(data[9:], uint32(b.version))
	o.Write(data, b.ops)
}

// Reset start reading from the op list.
func (r *OpsReader) Reset(ops *Ops) {
	r.ops = &ops.ops
	r.stack = r.stack[:0]
	r.pc = pc{}
}

func (r *OpsReader) Decode() (EncodedOp, bool) {
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
			return EncodedOp{}, false
		}
		key := OpKey{ops: r.ops, pc: r.pc.data, version: r.ops.version}
		t := ops.OpType(r.ops.data[r.pc.data])
		n := t.Size()
		nrefs := t.NumRefs()
		data := r.ops.data[r.pc.data : r.pc.data+n]
		refs := r.ops.refs[r.pc.refs : r.pc.refs+nrefs]
		switch t {
		case ops.TypeAux:
			var op opAux
			op.decode(data)
			n += op.len
			data = r.ops.data[r.pc.data : r.pc.data+n]
		case ops.TypeBlock:
			var op BlockOp
			op.decode(data, refs)
			blockOps := op.ops
			if ops.OpType(blockOps.data[op.pc.data]) != ops.TypeBlockDef {
				panic("invalid block reference")
			}
			if op.version != op.ops.version {
				panic("invalid BlockOp reference to reset Ops")
			}
			var opDef opBlockDef
			opDef.decode(blockOps.data[op.pc.data : op.pc.data+ops.TypeBlockDef.Size()])
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
			r.pc.data += ops.TypeBlockDef.Size()
			r.pc.refs += ops.TypeBlockDef.NumRefs()
			continue
		case ops.TypeBlockDef:
			var op opBlockDef
			op.decode(data)
			r.pc = op.endpc
			continue
		}
		r.pc.data += n
		r.pc.refs += nrefs
		return EncodedOp{Key: key, Data: data, Refs: refs}, true
	}
}
