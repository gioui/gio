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

	Config system.Config
	// By convention, a nil Queue is a signal to widgets to draw themselves
	// in a disabled state.
	Queue event.Queue
	*op.Ops
}

// NewContext is a shorthand for
//
//   Context{
//     Ops: ops,
//     Queue: q,
//     Config: cfg,
//     Constraints: Exact(size),
//   }
//
// NewContext calls ops.Reset.
func NewContext(ops *op.Ops, q event.Queue, cfg system.Config, size image.Point) Context {
	ops.Reset()
	return Context{
		Ops:         ops,
		Queue:       q,
		Config:      cfg,
		Constraints: Exact(size),
	}
}

// Now returns the configuration time or the zero time.
func (c Context) Now() time.Time {
	if c.Config == nil {
		return time.Time{}
	}
	return c.Config.Now()
}

// Px maps the value to pixels. If no configuration is set,
// Px returns the rounded value of v.
func (c Context) Px(v unit.Value) int {
	if c.Config == nil {
		return int(math.Round(float64(v.V)))
	}
	return c.Config.Px(v)
}

// Events returns the events available for the key. If no
// queue is configured, Events returns nil.
func (c Context) Events(k event.Tag) []event.Event {
	if c.Queue == nil {
		return nil
	}
	return c.Queue.Events(k)
}

// Disabled returns a copy of this context with a nil Queue,
// blocking events to widgets using it.
//
// By convention, a nil Queue is a signal to widgets to draw themselves
// in a disabled state.
func (c Context) Disabled() Context {
	c.Queue = nil
	return c
}
