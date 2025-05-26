// SPDX-License-Identifier: Unlicense OR MIT

// Package event contains types for event handling.
package event

import (
	"gioui.org/internal/ops"
	"gioui.org/op"
)

// Tag is the stable identifier for an event handler.
// For a handler h, the tag is typically &h.
type Tag any

// Event is the marker interface for events.
type Event interface {
	ImplementsEvent()
}

// Filter represents a filter for [Event] types.
type Filter interface {
	ImplementsFilter()
}

// Op declares a tag for input routing at the current transformation
// and clip area hierarchy. It panics if tag is nil.
func Op(o *op.Ops, tag Tag) {
	if tag == nil {
		panic("Tag must be non-nil")
	}
	data := ops.Write1(&o.Internal, ops.TypeInputLen, tag)
	data[0] = byte(ops.TypeInput)
}
