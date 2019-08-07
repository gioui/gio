package ui

import (
	"encoding/binary"

	"gioui.org/ui/internal/opconst"
)

// Ops holds a list of serialized Ops.
type Ops struct {
	Version int
	// Serialized ops.
	Data []byte
	// Op references.
	Refs []interface{}

	stackDepth int

	inAux  bool
	auxOff int
	auxLen int
}

type StackOp struct {
	depth  int
	active bool
	ops    *Ops
}

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
	for i := range o.Refs {
		o.Refs[i] = nil
	}
	o.Data = o.Data[:0]
	o.Refs = o.Refs[:0]
	o.Version++
}

// Internal use only.
func (o *Ops) Aux() []byte {
	if !o.inAux {
		return nil
	}
	return o.Data[o.auxOff+opconst.TypeAuxLen : o.auxOff+opconst.TypeAuxLen+o.auxLen]
}

func (d *Ops) write(op []byte, refs ...interface{}) {
	d.Data = append(d.Data, op...)
	d.Refs = append(d.Refs, refs...)
}

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
			bo.PutUint32(o.Data[o.auxOff+1:], uint32(o.auxLen))
		}
	}
	o.write(op, refs...)
}

func (d *Ops) pc() pc {
	return pc{data: len(d.Data), refs: len(d.Refs)}
}

// Record a macro of operations.
func (m *MacroOp) Record(o *Ops) {
	if m.recording {
		panic("already recording")
	}
	m.recording = true
	m.ops = o
	m.pc = o.pc()
	// Make room for a macro definition. Filled out in Stop.
	m.ops.Write(make([]byte, opconst.TypeMacroDefLen))
}

// Stop recording the macro.
func (m *MacroOp) Stop() {
	if !m.recording {
		panic("not recording")
	}
	m.recording = false
	pc := m.ops.pc()
	// Fill out the macro definition reserved in Record.
	data := m.ops.Data[m.pc.data : m.pc.data+opconst.TypeMacroDefLen]
	data[0] = byte(opconst.TypeMacroDef)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(pc.data))
	bo.PutUint32(data[5:], uint32(pc.refs))
	m.version = m.ops.Version
}

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
