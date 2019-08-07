// SPDX-License-Identifier: Unlicense OR MIT

package ops

import (
	"encoding/binary"

	"gioui.org/ui"
	"gioui.org/ui/internal/opconst"
)

// Reader parses an ops list.
type Reader struct {
	pc    pc
	stack []macro
	ops   *ui.Ops
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
	ops     *ui.Ops
	pc      int
	version int
}

// Shadow of ui.MacroOp.
type macroOp struct {
	recording bool
	ops       *ui.Ops
	version   int
	pc        pc
}

type pc struct {
	data int
	refs int
}

type macro struct {
	ops   *ui.Ops
	retPC pc
	endPC pc
}

type opMacroDef struct {
	endpc pc
}

type opAux struct {
	len int
}

// Reset start reading from the op list.
func (r *Reader) Reset(ops *ui.Ops) {
	r.stack = r.stack[:0]
	r.pc = pc{}
	r.ops = nil
	if ops == nil {
		return
	}
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
		if r.pc.data == len(r.ops.Data) {
			return EncodedOp{}, false
		}
		key := Key{ops: r.ops, pc: r.pc.data, version: r.ops.Version}
		t := opconst.OpType(r.ops.Data[r.pc.data])
		n := t.Size()
		nrefs := t.NumRefs()
		data := r.ops.Data[r.pc.data : r.pc.data+n]
		refs := r.ops.Refs[r.pc.refs : r.pc.refs+nrefs]
		switch t {
		case opconst.TypeAux:
			var op opAux
			op.decode(data)
			n += op.len
			data = r.ops.Data[r.pc.data : r.pc.data+n]
		case opconst.TypeMacro:
			var op macroOp
			op.decode(data, refs)
			macroOps := op.ops
			if opconst.OpType(macroOps.Data[op.pc.data]) != opconst.TypeMacroDef {
				panic("invalid macro reference")
			}
			if op.version != op.ops.Version {
				panic("invalid MacroOp reference to reset Ops")
			}
			var opDef opMacroDef
			opDef.decode(macroOps.Data[op.pc.data : op.pc.data+opconst.TypeMacroDef.Size()])
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

func (op *opAux) decode(data []byte) {
	if opconst.OpType(data[0]) != opconst.TypeAux {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	*op = opAux{
		len: int(int32(bo.Uint32(data[1:]))),
	}
}

func (m *macroOp) decode(data []byte, refs []interface{}) {
	if opconst.OpType(data[0]) != opconst.TypeMacro {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	dataIdx := int(int32(bo.Uint32(data[1:])))
	refsIdx := int(int32(bo.Uint32(data[5:])))
	version := int(int32(bo.Uint32(data[9:])))
	*m = macroOp{
		ops: refs[0].(*ui.Ops),
		pc: pc{
			data: dataIdx,
			refs: refsIdx,
		},
		version: version,
	}
}
