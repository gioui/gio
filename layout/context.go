// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/system"
	"gioui.org/op"
)

// Context carries the state needed by almost all layouts and widgets.
type Context struct {
	// Constraints track the constraints for the active widget or
	// layout.
	Constraints Constraints
	// Dimensions track the result of the most recent layout
	// operation.
	Dimensions Dimensions

	system.Config
	event.Queue
	*op.Ops
}

// layout a widget with a set of constraints and return its
// dimensions. The widget dimensions are constrained abd the previous
// constraints are restored after layout.
func ctxLayout(gtx *Context, cs Constraints, w Widget) Dimensions {
	saved := gtx.Constraints
	gtx.Constraints = cs
	gtx.Dimensions = Dimensions{}
	w()
	gtx.Dimensions.Size = cs.Constrain(gtx.Dimensions.Size)
	gtx.Constraints = saved
	return gtx.Dimensions
}

// Reset the context. The constraints' minimum and maximum values are
// set to the size.
func (c *Context) Reset(cfg system.Config, size image.Point) {
	c.Constraints = RigidConstraints(size)
	c.Dimensions = Dimensions{}
	c.Config = cfg
	if c.Ops == nil {
		c.Ops = new(op.Ops)
	}
	c.Ops.Reset()
}
