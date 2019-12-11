// SPDX-License-Identifier: Unlicense OR MIT

package ops

import (
	"encoding/binary"

	"gioui.org/internal/opconst"
	"gioui.org/op"
)

// Reader parses an ops list.
type Reader struct {
	pc    pc
	stack []macro
	ops   *op.Ops
}

// EncodedOp represents an encoded op returned by
// Reader.
type EncodedOp struct {
	Key  Key
	Data []byte
	Refs []interface{}
}

// Key is a unique key for a given op.
type Key struct {
	ops     *op.Ops
	pc      int
	version int
}

// Shadow of op.MacroOp.
type macroOp struct {
	pc pc
}

// Shadow of op.CallOp.
type callOp struct {
	ops *op.Ops
}

type pc struct {
	data int
	refs int
}

type macro struct {
	ops   *op.Ops
	retPC pc
	endPC pc
}

type opMacroDef struct {
	endpc pc
}

// Reset start reading from the op list.
func (r *Reader) Reset(ops *op.Ops) {
	r.stack = r.stack[:0]
	r.pc = pc{}
	r.ops = ops
}

func (r *Reader) Decode() (EncodedOp, bool) {
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
		data := r.ops.Data()
		data = data[r.pc.data:]
		if len(data) == 0 {
			return EncodedOp{}, false
		}
		key := Key{ops: r.ops, pc: r.pc.data, version: r.ops.Version()}
		t := opconst.OpType(data[0])
		n := t.Size()
		nrefs := t.NumRefs()
		data = data[:n]
		refs := r.ops.Refs()
		refs = refs[r.pc.refs:]
		refs = refs[:nrefs]
		switch t {
		case opconst.TypeAux:
			// An Aux operations is always wrapped in a macro, and
			// its length is the remaining space.
			block := r.stack[len(r.stack)-1]
			n += block.endPC.data - r.pc.data - opconst.TypeAuxLen
			data = data[:n]
		case opconst.TypeCall:
			var op callOp
			op.decode(data, refs)
			endPC := pc{
				data: len(op.ops.Data()),
				refs: len(op.ops.Refs()),
			}
			retPC := r.pc
			retPC.data += n
			retPC.refs += nrefs
			r.stack = append(r.stack, macro{
				ops:   r.ops,
				retPC: retPC,
				endPC: endPC,
			})
			r.pc = pc{}
			r.ops = op.ops
			continue
		case opconst.TypeMacro:
			var op macroOp
			op.decode(data)
			macroData := r.ops.Data()[op.pc.data:]
			if opconst.OpType(macroData[0]) != opconst.TypeMacroDef {
				panic("invalid macro reference")
			}
			var opDef opMacroDef
			opDef.decode(macroData[:opconst.TypeMacroDef.Size()])
			retPC := r.pc
			retPC.data += n
			retPC.refs += nrefs
			r.stack = append(r.stack, macro{
				ops:   r.ops,
				retPC: retPC,
				endPC: opDef.endpc,
			})
			r.pc = op.pc
			r.pc.data += opconst.TypeMacroDef.Size()
			r.pc.refs += opconst.TypeMacroDef.NumRefs()
			continue
		case opconst.TypeMacroDef:
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

func (op *opMacroDef) decode(data []byte) {
	if opconst.OpType(data[0]) != opconst.TypeMacroDef {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	dataIdx := int(int32(bo.Uint32(data[1:])))
	refsIdx := int(int32(bo.Uint32(data[5:])))
	*op = opMacroDef{
		endpc: pc{
			data: dataIdx,
			refs: refsIdx,
		},
	}
}

func (m *callOp) decode(data []byte, refs []interface{}) {
	if opconst.OpType(data[0]) != opconst.TypeCall {
		panic("invalid op")
	}
	*m = callOp{
		ops: refs[0].(*op.Ops),
	}
}

func (m *macroOp) decode(data []byte) {
	if opconst.OpType(data[0]) != opconst.TypeMacro {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	dataIdx := int(int32(bo.Uint32(data[1:])))
	refsIdx := int(int32(bo.Uint32(data[5:])))
	*m = macroOp{
		pc: pc{
			data: dataIdx,
			refs: refsIdx,
		},
	}
}
