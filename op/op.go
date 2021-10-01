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

An Ops list can be viewed as a very simple virtual machine: it has state such
as transformation and color and execution flow can be controlled with macros.

Some state, such as the current color, is modified directly by operations with
Add methods. Other state, such as transformation and clip shape, are
represented by stacks.

This example sets the simple color state and pushes an offset to the
transformation stack.

	ops := new(op.Ops)
	// Set the color.
	paint.ColorOp{...}.Add(ops)
	// Apply an offset to subsequent operations.
	stack := op.Offset(...).Push(ops)
	...
	// Undo the offset transformation.
	stack.Pop()

The MacroOp records a list of operations to be executed later:

	ops := new(op.Ops)
	macro := op.Record(ops)
	// Record operations by adding them.
	op.InvalidateOp{}.Add(ops)
	...
	// End recording.
	call := macro.Stop()

	// replay the recorded operations:
	call.Add(ops)

*/
package op

import (
	"encoding/binary"
	"image"
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
	// refs hold external references for operations.
	refs []interface{}
	// nextStateID is the id allocated for the next
	// StateOp.
	nextStateID int

	macroStack stack
	stacks     [3]stack
}

type StackKind uint8

const (
	ClipStack StackKind = iota
	AreaStack
	TransStack
)

// stateOp represents a saved operation snapshop to be restored
// later.
type stateOp struct {
	id      int
	macroID int
	ops     *Ops
}

// MacroOp records a list of operations for later use.
type MacroOp struct {
	ops *Ops
	id  StackID
	pc  pc
}

// CallOp invokes the operations recorded by Record.
type CallOp struct {
	// Ops is the list of operations to invoke.
	ops *Ops
	pc  pc
}

// InvalidateOp requests a redraw at the given time. Use
// the zero value to request an immediate redraw.
type InvalidateOp struct {
	At time.Time
}

// TransformOp represents a transformation that can be pushed on the
// transformation stack.
type TransformOp struct {
	t f32.Affine2D
}

// TransformStack represents a TransformOp pushed on the transformation stack.
type TransformStack struct {
	id      StackID
	macroID int
	ops     *Ops
}

// stack tracks the integer identities of stack operations to ensure correct
// pairing of their push and pop methods.
type stack struct {
	currentID int
	nextID    int
}

type StackID struct {
	id   int
	prev int
}

type pc struct {
	data int
	refs int
}

// Defer executes c after all other operations have completed, including
// previously deferred operations.
// Defer saves the transformation stack and pushes it prior to executing
// c. All other operation state is reset.
//
// Note that deferred operations are executed in first-in-first-out order,
// unlike the Go facility of the same name.
func Defer(o *Ops, c CallOp) {
	if c.ops == nil {
		return
	}
	state := save(o)
	// Wrap c in a macro that loads the saved state before execution.
	m := Record(o)
	state.load()
	c.Add(o)
	c = m.Stop()
	// A Defer is recorded as a TypeDefer followed by the
	// wrapped macro.
	data := o.Write(opconst.TypeDeferLen)
	data[0] = byte(opconst.TypeDefer)
	c.Add(o)
}

type SaveStack struct {
	ops  *Ops
	clip struct {
		id      StackID
		macroID int
	}
	trans TransformStack
}

// Deprecated: use state-specific stack operations instead (TransformOp.Push
// etc.).
func Save(o *Ops) SaveStack {
	st := SaveStack{
		ops:   o,
		trans: Offset(f32.Point{}).Push(o),
	}
	const inf = 1e6
	bounds := image.Rectangle{Min: image.Pt(-inf, -inf), Max: image.Pt(inf, inf)}
	{
		st.clip.id, st.clip.macroID = o.PushOp(ClipStack)
		// Push clip stack with no-op (infinite) clipping rect. Copied from clip.Op.Push.
		bo := binary.LittleEndian
		data := o.Write(opconst.TypeClipLen)
		data[0] = byte(opconst.TypeClip)
		bo.PutUint32(data[1:], uint32(bounds.Min.X))
		bo.PutUint32(data[5:], uint32(bounds.Min.Y))
		bo.PutUint32(data[9:], uint32(bounds.Max.X))
		bo.PutUint32(data[13:], uint32(bounds.Max.Y))
		data[17] = byte(1) // Outline
		data[18] = byte(1) // Push
	}
	return st
}

func (s SaveStack) Load() {
	// Pop clip.
	s.ops.PopOp(ClipStack, s.clip.id, s.clip.macroID)
	data := s.ops.Write(opconst.TypePopClipLen)
	data[0] = byte(opconst.TypePopClip)

	s.trans.Pop()
}

// save the effective transformation.
func save(o *Ops) stateOp {
	o.nextStateID++
	s := stateOp{
		ops:     o,
		id:      o.nextStateID,
		macroID: o.macroStack.currentID,
	}
	bo := binary.LittleEndian
	data := o.Write(opconst.TypeSaveLen)
	data[0] = byte(opconst.TypeSave)
	bo.PutUint32(data[1:], uint32(s.id))
	return s
}

// load a previously saved operations state given
// its ID.
func (s stateOp) load() {
	bo := binary.LittleEndian
	data := s.ops.Write(opconst.TypeLoadLen)
	data[0] = byte(opconst.TypeLoad)
	bo.PutUint32(data[1:], uint32(s.id))
}

// Reset the Ops, preparing it for re-use. Reset invalidates
// any recorded macros.
func (o *Ops) Reset() {
	o.macroStack = stack{}
	for i := range o.stacks {
		o.stacks[i] = stack{}
	}
	// Leave references to the GC.
	for i := range o.refs {
		o.refs[i] = nil
	}
	o.data = o.data[:0]
	o.refs = o.refs[:0]
	o.nextStateID = 0
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
func (o *Ops) Write(n int) []byte {
	o.data = append(o.data, make([]byte, n)...)
	return o.data[len(o.data)-n:]
}

func (o *Ops) PushOp(kind StackKind) (StackID, int) {
	return o.stacks[kind].push(), o.macroStack.currentID
}

func (o *Ops) PopOp(kind StackKind, sid StackID, macroID int) {
	if o.macroStack.currentID != macroID {
		panic("stack push and pop must not cross macro boundary")
	}
	o.stacks[kind].pop(sid)
}

// Write1 is for internal use only.
func (o *Ops) Write1(n int, ref1 interface{}) []byte {
	o.data = append(o.data, make([]byte, n)...)
	o.refs = append(o.refs, ref1)
	return o.data[len(o.data)-n:]
}

// Write2 is for internal use only.
func (o *Ops) Write2(n int, ref1, ref2 interface{}) []byte {
	o.data = append(o.data, make([]byte, n)...)
	o.refs = append(o.refs, ref1, ref2)
	return o.data[len(o.data)-n:]
}

func (o *Ops) pc() pc {
	return pc{data: len(o.data), refs: len(o.refs)}
}

// Record a macro of operations.
func Record(o *Ops) MacroOp {
	m := MacroOp{
		ops: o,
		id:  o.macroStack.push(),
		pc:  o.pc(),
	}
	// Reserve room for a macro definition. Updated in Stop.
	m.ops.Write(opconst.TypeMacroLen)
	m.fill()
	return m
}

// Stop ends a previously started recording and returns an
// operation for replaying it.
func (m MacroOp) Stop() CallOp {
	m.ops.macroStack.pop(m.id)
	m.fill()
	return CallOp{
		ops: m.ops,
		pc:  m.pc,
	}
}

func (m MacroOp) fill() {
	pc := m.ops.pc()
	// Fill out the macro definition reserved in Record.
	data := m.ops.data[m.pc.data:]
	data = data[:opconst.TypeMacroLen]
	data[0] = byte(opconst.TypeMacro)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(pc.data))
	bo.PutUint32(data[5:], uint32(pc.refs))
}

// Add the recorded list of operations. Add
// panics if the Ops containing the recording
// has been reset.
func (c CallOp) Add(o *Ops) {
	if c.ops == nil {
		return
	}
	data := o.Write1(opconst.TypeCallLen, c.ops)
	data[0] = byte(opconst.TypeCall)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(c.pc.data))
	bo.PutUint32(data[5:], uint32(c.pc.refs))
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

// Offset creates a TransformOp with the offset o.
func Offset(o f32.Point) TransformOp {
	return TransformOp{t: f32.Affine2D{}.Offset(o)}
}

// Affine creates a TransformOp representing the transformation a.
func Affine(a f32.Affine2D) TransformOp {
	return TransformOp{t: a}
}

// Push the current transformation to the stack and then multiply the
// current transformation with t.
func (t TransformOp) Push(o *Ops) TransformStack {
	id, macroID := o.PushOp(TransStack)
	t.add(o, true)
	return TransformStack{ops: o, id: id, macroID: macroID}
}

// Add is like Push except it doesn't push the current transformation to the
// stack.
func (t TransformOp) Add(o *Ops) {
	t.add(o, false)
}

func (t TransformOp) add(o *Ops, push bool) {
	data := o.Write(opconst.TypeTransformLen)
	data[0] = byte(opconst.TypeTransform)
	if push {
		data[1] = 1
	}
	bo := binary.LittleEndian
	a, b, c, d, e, f := t.t.Elems()
	bo.PutUint32(data[2:], math.Float32bits(a))
	bo.PutUint32(data[2+4*1:], math.Float32bits(b))
	bo.PutUint32(data[2+4*2:], math.Float32bits(c))
	bo.PutUint32(data[2+4*3:], math.Float32bits(d))
	bo.PutUint32(data[2+4*4:], math.Float32bits(e))
	bo.PutUint32(data[2+4*5:], math.Float32bits(f))
}

func (t TransformStack) Pop() {
	t.ops.PopOp(TransStack, t.id, t.macroID)
	data := t.ops.Write(opconst.TypePopTransformLen)
	data[0] = byte(opconst.TypePopTransform)
}

func (s *stack) push() StackID {
	s.nextID++
	sid := StackID{
		id:   s.nextID,
		prev: s.currentID,
	}
	s.currentID = s.nextID
	return sid
}

func (s *stack) check(sid StackID) {
	if s.currentID != sid.id {
		panic("unbalanced operation")
	}
}

func (s *stack) pop(sid StackID) {
	s.check(sid)
	s.currentID = sid.prev
}
