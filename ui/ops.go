package ui

import (
	"encoding/binary"
	"errors"
	"fmt"

	"gioui.org/ui/internal/ops"
)

// Ops holds a list of serialized Ops.
type Ops struct {
	// Stack of macro start indices.
	stack   []pc
	version int
	// Serialized ops.
	data []byte
	// Op references.
	refs []interface{}

	inAux  bool
	auxOff int
	auxLen int
}

// OpsReader parses an ops list. Internal use only.
type OpsReader struct {
	pc    pc
	stack []macro
	ops   *Ops
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
	ops     *Ops
	pc      int
	version int
}

type macro struct {
	ops   *Ops
	retPC pc
	endPC pc
}

type pc struct {
	data int
	refs int
}

type PushOp struct{}

type PopOp struct{}

type MacroOp struct {
	ops     *Ops
	version int
	pc      pc
}

type opMacroDef struct {
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

// Record starts recording a macro. Multiple simultaneous
// recordings are supported. Stop ends the most recent.
func (o *Ops) Record() {
	o.stack = append(o.stack, o.pc())
	// Make room for a macro definition. Filled out in Stop.
	o.Write(make([]byte, ops.TypeMacroDefLen))
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

func (op *opMacroDef) decode(data []byte) {
	if ops.OpType(data[0]) != ops.TypeMacroDef {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	dataIdx := int(bo.Uint32(data[1:]))
	refsIdx := int(bo.Uint32(data[5:]))
	*op = opMacroDef{
		endpc: pc{
			data: dataIdx,
			refs: refsIdx,
		},
	}
}

// Stop the most recent recording and return the macro for later
// use.
func (o *Ops) Stop() MacroOp {
	if len(o.stack) == 0 {
		panic(errors.New("not recording a macro"))
	}
	start := o.stack[len(o.stack)-1]
	o.stack = o.stack[:len(o.stack)-1]
	pc := o.pc()
	// Write the macro header reserved in Begin.
	data := o.data[start.data : start.data+ops.TypeMacroDefLen]
	data[0] = byte(ops.TypeMacroDef)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(pc.data))
	bo.PutUint32(data[5:], uint32(pc.refs))
	return MacroOp{ops: o, pc: start, version: o.version}
}

// Reset the Ops, preparing it for re-use.
func (o *Ops) Reset() {
	o.inAux = false
	o.stack = o.stack[:0]
	// Leave references to the GC.
	for i := range o.refs {
		o.refs[i] = nil
	}
	o.data = o.data[:0]
	o.refs = o.refs[:0]
	o.version++
}

// Internal use only.
func (o *Ops) Aux() []byte {
	if !o.inAux {
		return nil
	}
	return o.data[o.auxOff+ops.TypeAuxLen : o.auxOff+ops.TypeAuxLen+o.auxLen]
}

func (d *Ops) write(op []byte, refs ...interface{}) {
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
			o.auxOff = o.pc().data
			o.auxLen = 0
			header := make([]byte, ops.TypeAuxLen)
			header[0] = byte(ops.TypeAux)
			o.write(header)
		}
		o.auxLen += len(op)
	default:
		if o.inAux {
			o.inAux = false
			bo := binary.LittleEndian
			bo.PutUint32(o.data[o.auxOff+1:], uint32(o.auxLen))
		}
	}
	o.write(op, refs...)
}

func (d *Ops) pc() pc {
	return pc{data: len(d.data), refs: len(d.refs)}
}

func (b *MacroOp) decode(data []byte, refs []interface{}) {
	if ops.OpType(data[0]) != ops.TypeMacro {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	dataIdx := int(bo.Uint32(data[1:]))
	refsIdx := int(bo.Uint32(data[5:]))
	version := int(bo.Uint32(data[9:]))
	*b = MacroOp{
		ops: refs[0].(*Ops),
		pc: pc{
			data: dataIdx,
			refs: refsIdx,
		},
		version: version,
	}
}

func (b MacroOp) Add(o *Ops) {
	if b.ops == nil {
		return
	}
	data := make([]byte, ops.TypeMacroLen)
	data[0] = byte(ops.TypeMacro)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(b.pc.data))
	bo.PutUint32(data[5:], uint32(b.pc.refs))
	bo.PutUint32(data[9:], uint32(b.version))
	o.Write(data, b.ops)
}

// Reset start reading from the op list.
func (r *OpsReader) Reset(ops *Ops) {
	r.stack = r.stack[:0]
	r.pc = pc{}
	r.ops = nil
	if ops == nil {
		return
	}
	if n := len(ops.stack); n > 0 {
		panic(fmt.Errorf("%d Begin(s) not matched with End", n))
	}
	r.ops = ops
}

func (r *OpsReader) Decode() (EncodedOp, bool) {
	if r.ops == nil {
		return EncodedOp{}, false
	}
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
		case ops.TypeMacro:
			var op MacroOp
			op.decode(data, refs)
			macroOps := op.ops
			if ops.OpType(macroOps.data[op.pc.data]) != ops.TypeMacroDef {
				panic("invalid macro reference")
			}
			if op.version != op.ops.version {
				panic("invalid MacroOp reference to reset Ops")
			}
			var opDef opMacroDef
			opDef.decode(macroOps.data[op.pc.data : op.pc.data+ops.TypeMacroDef.Size()])
			retPC := r.pc
			retPC.data += n
			retPC.refs += nrefs
			r.stack = append(r.stack, macro{
				ops:   r.ops,
				retPC: retPC,
				endPC: opDef.endpc,
			})
			r.ops = macroOps
			r.pc = op.pc
			r.pc.data += ops.TypeMacroDef.Size()
			r.pc.refs += ops.TypeMacroDef.NumRefs()
			continue
		case ops.TypeMacroDef:
			var op opMacroDef
			op.decode(data)
			r.pc = op.endpc
			continue
		}
		r.pc.data += n
		r.pc.refs += nrefs
		return EncodedOp{Key: key, Data: data, Refs: refs}, true
	}
}
