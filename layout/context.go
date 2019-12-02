// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"
	"math"
	"time"

	"gioui.org/io/event"
	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/unit"
)

// Context carries the state needed by almost all layouts and widgets.
// A zero value Context never returns events, map units to pixels
// with a scale of 1.0, and returns the zero time from Now.
type Context struct {
	// Constraints track the constraints for the active widget or
	// layout.
	Constraints Constraints
	// Dimensions track the result of the most recent layout
	// operation.
	Dimensions Dimensions

	cfg   system.Config
	queue event.Queue
	*op.Ops
}

// NewContext returns a Context for an event queue.
func NewContext(q event.Queue) *Context {
	return &Context{
		queue: q,
	}
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
	c.cfg = cfg
	if c.Ops == nil {
		c.Ops = new(op.Ops)
	}
	c.Ops.Reset()
}

// Now returns the configuration time or the the zero time.
func (c *Context) Now() time.Time {
	if c.cfg == nil {
		return time.Time{}
	}
	return c.cfg.Now()
}

// Px maps the value to pixels. If no configuration is set,
// Px returns the rounded value of v.
func (c *Context) Px(v unit.Value) int {
	if c.cfg == nil {
		return int(math.Round(float64(v.V)))
	}
	return c.cfg.Px(v)
}

// Events returns the events available for the key. If no
// queue is configured, Events returns nil.
func (c *Context) Events(k event.Key) []event.Event {
	if c.queue == nil {
		return nil
	}
	return c.queue.Events(k)
}
