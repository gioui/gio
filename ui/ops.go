// SPDX-License-Identifier: Unlicense OR MIT

package ui

import (
	"encoding/binary"

	"gioui.org/ui/internal/opconst"
)

// Ops holds a list of operations. Operations are stored in
// serialized form to avoid garbage during construction of
// the ops list.
type Ops struct {
	// version is incremented at each Reset.
	version int
	// data contains the serialized operations.
	data []byte
	// External references for operations.
	refs []interface{}

	stackDepth int

	inAux  bool
	auxOff int
	auxLen int
}

// StackOp can save and restore the operation state
// in a stack-like manner.
type StackOp struct {
	depth  int
	active bool
	ops    *Ops
}

// MacroOp can record a list of operations for later
// use.
type MacroOp struct {
	recording bool
	ops       *Ops
	version   int
	pc        pc
}

type pc struct {
	data int
	refs int
}

// Push (save) the current operations state.
func (s *StackOp) Push(o *Ops) {
	if s.active {
		panic("unbalanced push")
	}
	s.active = true
	s.ops = o
	o.stackDepth++
	s.depth = o.stackDepth
	o.Write([]byte{byte(opconst.TypePush)})
}

// Pop (restore) a previously Pushed operations state.
func (s *StackOp) Pop() {
	if !s.active {
		panic("unbalanced pop")
	}
	d := s.ops.stackDepth
	if d != s.depth {
		panic("unbalanced pop")
	}
	s.active = false
	s.ops.stackDepth--
	s.ops.Write([]byte{byte(opconst.TypePop)})
}

// Reset the Ops, preparing it for re-use.
func (o *Ops) Reset() {
	o.inAux = false
	o.stackDepth = 0
	// Leave references to the GC.
	for i := range o.refs {
		o.refs[i] = nil
	}
	o.data = o.data[:0]
	o.refs = o.refs[:0]
	o.version++
}

// Internal use only.
func (o *Ops) Data() []byte {
	return o.data
}

// Internal use only.
func (o *Ops) Refs() []interface{} {
	return o.refs
}

// Internal use only.
func (o *Ops) Version() int {
	return o.version
}

// Internal use only.
func (o *Ops) Aux() []byte {
	if !o.inAux {
		return nil
	}
	aux := o.data[o.auxOff+opconst.TypeAuxLen:]
	return aux[:o.auxLen]
}

func (d *Ops) write(op []byte, refs ...interface{}) {
	d.data = append(d.data, op...)
	d.refs = append(d.refs, refs...)
}

// Internal use only.
func (o *Ops) Write(op []byte, refs ...interface{}) {
	t := opconst.OpType(op[0])
	if len(refs) != t.NumRefs() {
		panic("invalid ref count")
	}
	switch t {
	case opconst.TypeAux:
		// Write only the data.
		op = op[1:]
		if !o.inAux {
			o.inAux = true
			o.auxOff = o.pc().data
			o.auxLen = 0
			header := make([]byte, opconst.TypeAuxLen)
			header[0] = byte(opconst.TypeAux)
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

// Record a macro of operations.
func (m *MacroOp) Record(o *Ops) {
	if m.recording {
		panic("already recording")
	}
	m.recording = true
	m.ops = o
	m.pc = o.pc()
	// Reserve room for a macro definition. Updated in Stop.
	m.ops.Write(make([]byte, opconst.TypeMacroDefLen))
	m.fill()
}

// Stop ends a previously started recording.
func (m *MacroOp) Stop() {
	if !m.recording {
		panic("not recording")
	}
	m.recording = false
	m.fill()
}

func (m *MacroOp) fill() {
	pc := m.ops.pc()
	// Fill out the macro definition reserved in Record.
	data := m.ops.data[m.pc.data:]
	data = data[:opconst.TypeMacroDefLen]
	data[0] = byte(opconst.TypeMacroDef)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(pc.data))
	bo.PutUint32(data[5:], uint32(pc.refs))
	m.version = m.ops.version
}

// Add the recorded list of operations. The Ops
// argument may be different than the Ops argument
// passed to Record.
func (m MacroOp) Add(o *Ops) {
	if m.recording {
		panic("a recording is in progress")
	}
	if m.ops == nil {
		return
	}
	data := make([]byte, opconst.TypeMacroLen)
	data[0] = byte(opconst.TypeMacro)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(m.pc.data))
	bo.PutUint32(data[5:], uint32(m.pc.refs))
	bo.PutUint32(data[9:], uint32(m.version))
	o.Write(data, m.ops)
}
