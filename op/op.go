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

# State

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
	...
	// End recording.
	call := macro.Stop()

	// replay the recorded operations:
	call.Add(ops)
*/
package op

import (
	"encoding/binary"
	"fmt"
	"image"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/internal/ops"
)

// Ops holds a list of operations. Operations are stored in
// serialized form to avoid garbage during construction of
// the ops list.
type Ops struct {
	// Internal is for internal use, despite being exported.
	Internal ops.Ops
}

// MacroOp records a list of operations for later use.
type MacroOp struct {
	ops *ops.Ops
	id  ops.StackID
	pc  ops.PC
}

// CallOp invokes the operations recorded by Record.
type CallOp struct {
	// Ops is the list of operations to invoke.
	ops   *ops.Ops
	start ops.PC
	end   ops.PC
}

// InvalidateCmd requests a redraw at the given time. Use
// the zero value to request an immediate redraw.
type InvalidateCmd struct {
	At time.Time
}

// TransformOp represents a transformation that can be pushed on the
// transformation stack.
type TransformOp struct {
	t f32.Affine2D
}

// TransformStack represents a TransformOp pushed on the transformation stack.
type TransformStack struct {
	id      ops.StackID
	macroID uint32
	ops     *ops.Ops
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
	fmt.Println("Deferring active..")
	state := ops.Save(&o.Internal)
	// Wrap c in a macro that loads the saved state before execution.
	m := Record(o)
	state.Load()
	c.Add(o)
	c = m.Stop()
	// A Defer is recorded as a TypeDefer followed by the
	// wrapped macro.
	data := ops.Write(&o.Internal, ops.TypeDeferLen)
	data[0] = byte(ops.TypeDefer)
	c.Add(o)
}

func Defer2(o *Ops, c CallOp) {
	if c.ops == nil {
		return
	}
	state := ops.Save(&o.Internal)
	// Wrap c in a macro that loads the saved state before execution.
	m := Record(o)
	state.Load()
	c.Add(o)
	c = m.Stop()
	// A Defer is recorded as a TypeDefer followed by the
	// wrapped macro.
	data := ops.Write(&o.Internal, ops.TypeDeferLen)
	data[0] = byte(ops.TypeDefer)
	c.Add(o)
}

// Reset the Ops, preparing it for re-use. Reset invalidates
// any recorded macros.
func (o *Ops) Reset() {
	ops.Reset(&o.Internal)
}

// Record a macro of operations.
func Record(o *Ops) MacroOp {
	m := MacroOp{
		ops: &o.Internal,
		id:  ops.PushMacro(&o.Internal),
		pc:  ops.PCFor(&o.Internal),
	}
	// Reserve room for a macro definition. Updated in Stop.
	data := ops.Write(m.ops, ops.TypeMacroLen)
	data[0] = byte(ops.TypeMacro)
	return m
}

// Stop ends a previously started recording and returns an
// operation for replaying it.
func (m MacroOp) Stop() CallOp {
	ops.PopMacro(m.ops, m.id)
	ops.FillMacro(m.ops, m.pc)
	return CallOp{
		ops: m.ops,
		// Skip macro header.
		start: m.pc.Add(ops.TypeMacro),
		end:   ops.PCFor(m.ops),
	}
}

// Add the recorded list of operations. Add
// panics if the Ops containing the recording
// has been reset.
func (c CallOp) Add(o *Ops) {
	if c.ops == nil {
		return
	}
	ops.AddCall(&o.Internal, c.ops, c.start, c.end)
}

// Offset converts an offset to a TransformOp.
func Offset(off image.Point) TransformOp {
	offf := f32.Pt(float32(off.X), float32(off.Y))
	return Affine(f32.Affine2D{}.Offset(offf))
}

// Affine creates a TransformOp representing the transformation a.
func Affine(a f32.Affine2D) TransformOp {
	return TransformOp{t: a}
}

// Push the current transformation to the stack and then multiply the
// current transformation with t.
func (t TransformOp) Push(o *Ops) TransformStack {
	id, macroID := ops.PushOp(&o.Internal, ops.TransStack)
	t.add(o, true)
	return TransformStack{ops: &o.Internal, id: id, macroID: macroID}
}

// Add is like Push except it doesn't push the current transformation to the
// stack.
func (t TransformOp) Add(o *Ops) {
	t.add(o, false)
}

func (t TransformOp) add(o *Ops, push bool) {
	data := ops.Write(&o.Internal, ops.TypeTransformLen)
	data[0] = byte(ops.TypeTransform)
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
	ops.PopOp(t.ops, ops.TransStack, t.id, t.macroID)
	data := ops.Write(t.ops, ops.TypePopTransformLen)
	data[0] = byte(ops.TypePopTransform)
}

func (InvalidateCmd) ImplementsCommand() {}
