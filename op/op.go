// SPDX-License-Identifier: Unlicense OR MIT

/*

Package op implements operations for updating a user interface.

Gio programs use operations, or ops, for describing their user
interfaces. There are operations for drawing, defining input
handlers, changing window properties as well as operations for
controlling the execution of other operations.

Ops represents a list of operations. The most important use
for an Ops list is to describe a complete user interface update
to a ui/app.Window's Update method.

Drawing a colored square:

	import "gioui.org/unit"
	import "gioui.org/app"
	import "gioui.org/op/paint"

	var w app.Window
	var e system.FrameEvent
	ops := new(op.Ops)
	...
	ops.Reset()
	paint.ColorOp{Color: ...}.Add(ops)
	paint.PaintOp{Rect: ...}.Add(ops)
	e.Frame(ops)

State

An Ops list can be viewed as a very simple virtual machine: it has an implicit
mutable state stack and execution flow can be controlled with macros.

The StackOp saves the current state to the state stack and restores it later:

	ops := new(op.Ops)
	var stack op.StackOp
	// Save the current state, in particular the transform.
	stack.Push(ops)
	// Apply a transform to subsequent operations.
	op.TransformOp{}.Offset(...).Add(ops)
	...
	// Restore the previous transform.
	stack.Pop()

The CallOp invokes another operation list:

	ops := new(op.Ops)
	ops2 := new(op.Ops)
	op.CallOp{Ops: ops2}.Add(ops)

The MacroOp records a list of operations to be executed later:

	ops := new(op.Ops)
	var macro op.MacroOp
	macro.Record(ops)
	// Record operations by adding them.
	op.InvalidateOp{}.Add(ops)
	...
	// End recording.
	macro.Stop()

	// replay the recorded operations by calling Add:
	macro.Add()

*/
package op

import (
	"encoding/binary"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/internal/opconst"
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

	stackStack stack
	macroStack stack
}

// StackOp saves and restores the operation state
// in a stack-like manner.
type StackOp struct {
	stackDepth int
	id         stackID
	macroID    int
	active     bool
	ops        *Ops
}

// MacroOp records a list of operations for later use.
type MacroOp struct {
	recording bool
	ops       *Ops
	id        stackID
	pc        pc
}

// CallOp invokes all the operations from a separate
// operations list.
type CallOp struct {
	// Ops is the list of operations to invoke.
	Ops *Ops
}

// InvalidateOp requests a redraw at the given time. Use
// the zero value to request an immediate redraw.
type InvalidateOp struct {
	At time.Time
}

// TransformOp applies a transform to the current transform.
type TransformOp struct {
	// TODO: general transformations.
	offset f32.Point
}

// stack tracks the integer identities of StackOp and MacroOp
// operations to ensure correct pairing of Push/Pop and Record/End.
type stack struct {
	currentID int
	nextID    int
}

type stackID struct {
	id   int
	prev int
}

type pc struct {
	data int
	refs int
}

// Add the call to the operation list.
func (c CallOp) Add(o *Ops) {
	if c.Ops == nil {
		return
	}
	data := o.Write(opconst.TypeCallLen, c.Ops)
	data[0] = byte(opconst.TypeCall)
}

// Push (save) the current operations state.
func (s *StackOp) Push(o *Ops) {
	if s.active {
		panic("unbalanced push")
	}
	s.active = true
	s.ops = o
	s.id = o.stackStack.push()
	s.macroID = o.macroStack.currentID
	data := o.Write(opconst.TypePushLen)
	data[0] = byte(opconst.TypePush)
}

// Pop (restore) a previously Pushed operations state.
func (s *StackOp) Pop() {
	if !s.active {
		panic("unbalanced pop")
	}
	if s.ops.macroStack.currentID != s.macroID {
		panic("pop in a different macro than push")
	}
	s.ops.stackStack.pop(s.id)
	s.active = false
	data := s.ops.Write(opconst.TypePopLen)
	data[0] = byte(opconst.TypePop)
}

// Reset the Ops, preparing it for re-use.
func (o *Ops) Reset() {
	o.stackStack = stack{}
	o.macroStack = stack{}
	// Leave references to the GC.
	for i := range o.refs {
		o.refs[i] = nil
	}
	o.data = o.data[:0]
	o.refs = o.refs[:0]
	o.version++
}

// Data is for internal use only.
func (o *Ops) Data() []byte {
	return o.data
}

// Refs is for internal use only.
func (o *Ops) Refs() []interface{} {
	return o.refs
}

// Version is for internal use only.
func (o *Ops) Version() int {
	return o.version
}

// Write is for internal use only.
func (o *Ops) Write(n int, refs ...interface{}) []byte {
	o.data = append(o.data, make([]byte, n)...)
	o.refs = append(o.refs, refs...)
	return o.data[len(o.data)-n:]
}

func (o *Ops) pc() pc {
	return pc{data: len(o.data), refs: len(o.refs)}
}

// Record a macro of operations.
func (m *MacroOp) Record(o *Ops) {
	if m.recording {
		panic("already recording")
	}
	m.recording = true
	m.ops = o
	m.id = m.ops.macroStack.push()
	m.pc = o.pc()
	// Reserve room for a macro definition. Updated in Stop.
	m.ops.Write(opconst.TypeMacroDefLen)
	m.fill()
}

// Stop ends a previously started recording.
func (m *MacroOp) Stop() {
	if !m.recording {
		panic("not recording")
	}
	m.ops.macroStack.pop(m.id)
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
}

// Add the recorded list of operations.
func (m *MacroOp) Add() {
	if m.recording {
		panic("a recording is in progress")
	}
	if m.ops == nil {
		return
	}
	data := m.ops.Write(opconst.TypeMacroLen)
	data[0] = byte(opconst.TypeMacro)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(m.pc.data))
	bo.PutUint32(data[5:], uint32(m.pc.refs))
}

func (r InvalidateOp) Add(o *Ops) {
	data := o.Write(opconst.TypeRedrawLen)
	data[0] = byte(opconst.TypeInvalidate)
	bo := binary.LittleEndian
	// UnixNano cannot represent the zero time.
	if t := r.At; !t.IsZero() {
		nanos := t.UnixNano()
		if nanos > 0 {
			bo.PutUint64(data[1:], uint64(nanos))
		}
	}
}

// Offset the transformation.
func (t TransformOp) Offset(o f32.Point) TransformOp {
	return t.Multiply(TransformOp{o})
}

// Invert the transformation.
func (t TransformOp) Invert() TransformOp {
	return TransformOp{offset: t.offset.Mul(-1)}
}

// Transform a point.
func (t TransformOp) Transform(p f32.Point) f32.Point {
	return p.Add(t.offset)
}

// Multiply by a transformation.
func (t TransformOp) Multiply(t2 TransformOp) TransformOp {
	return TransformOp{
		offset: t.offset.Add(t2.offset),
	}
}

func (t TransformOp) Add(o *Ops) {
	data := o.Write(opconst.TypeTransformLen)
	data[0] = byte(opconst.TypeTransform)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], math.Float32bits(t.offset.X))
	bo.PutUint32(data[5:], math.Float32bits(t.offset.Y))
}

func (s *stack) push() stackID {
	s.nextID++
	sid := stackID{
		id:   s.nextID,
		prev: s.currentID,
	}
	s.currentID = s.nextID
	return sid
}

func (s *stack) check(sid stackID) {
	if s.currentID != sid.id {
		panic("unbalanced operation")
	}
}

func (s *stack) pop(sid stackID) {
	s.check(sid)
	s.currentID = sid.prev
}
