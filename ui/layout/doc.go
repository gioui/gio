// SPDX-License-Identifier: Unlicense OR MIT

/*
Package layout implements layouts common to GUI programs.

Constraints and dimensions

Constraints and dimensions form the the interface between
layouts and interface child elements. Every layout operation
start with a set of constraints for acceptable widths and heights
of a child. The operation ends by the child computing and returning
its chosen size in the form of a Dimens.

For example, to add space above a widget:

	var cs layout.Constraints = ...

	// Configure a top inset.
	inset := layout.Inset{Top: ui.Dp(8), ...}
	// Start insetting and modify the constraints.
	cs = inset.Begin(..., cs)
	// Lay out widget and determine its size given the constraints.
	dimensions := widget.Layout(..., cs)
	// End the inset and account for the insets.
	dimensions = inset.End(dimensions)

Note that the example does not generate any garbage even though the
Inset is transient. Layouts that don't accept user input are designed
to escape to the heap during their use.

Layout operations are recursive: a child in a layout operation can
itself be another layout. That way, complex user interfaces can
be created from a few generic layouts.

This example both aligns and insets a child:

	inset := layout.Inset{...}
	cs = inset.Begin(..., cs)
	align := layout.Align{...}
	cs = align.Begin(..., cs)
	dims := widget.Layout(..., cs)
	dims = align.End(dims)
	dims = inset.End(dims)

More complex layouts such as Stack and Flex lay out multiple children,
and stateful layouts such as List accept user input.

*/
package layout
