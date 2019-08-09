// SPDX-License-Identifier: Unlicense OR MIT

/*
Package ui defines operations buffers, units and common operations
for GUI programs written with the Gio module.

See https://gioui.org for instructions to setup and run Gio programs.

Operations

Gio programs use operations, or ops, for describing their user
interfaces. There are operations for drawing, defining input
handlers, changing window properties as well as operations for
controlling the execution of other operations.

Ops represents a list of operations. The most important use
for an Ops list is to describe a complete user interface update
to a ui/app.Window's Update method.

Drawing a colored square:

	import "gioui.org/ui"
	import "gioui.org/ui/app"
	import "gioui.org/ui/paint"

	var w app.Window
	var ops ui.Ops
	...
	ops.Reset()
	paint.ColorOp{Color: ...}.Add(ops)
	paint.PaintOp{Rect: ...}.Add(ops)
	w.Update(ops)

State

An Ops list can be viewed as a very simple virtual machine: it has an implicit
mutable state stack and execution flow can be controlled with macros.

The StackOp saves the current state to the state stack and restores it later:

	var ops ui.Ops
	var stack ui.StackOp
	// Save the current state, in particular the transform.
	stack.Push(ops)
	// Apply a transform to subsequent operations.
	ui.TransformOp{}.Offset(...).Add(ops)
	...
	// Restore the previous transform.
	stack.Pop()

The MacroOp records a list of operations to be executed later:

	var ops ui.Ops
	var macro ui.MacroOp
	macro.Record()
	// Record operations by adding them.
	ui.InvalidateOp{}.Add(ops)
	...
	macro.Stop()
	...
	// Execute the recorded operations.
	macro.Add(ops)

Note that operations added between Record and Stop are not executed until
the macro is Added.

Units

A Value is a value with a Unit attached.

Device independent pixel, or dp, is the unit for sizes independent of
the underlying display device.

Scaled pixels, or sp, is the unit for text sizes. An sp is like dp with
text scaling applied.

Finally, pixels, or px, is the unit for display dependent pixels. Their
size vary between platforms and displays.

To maintain a constant visual size across platforms and displays, always
use dps or sps to define user interfaces. Only use pixels for derived
values.
*/
package ui
