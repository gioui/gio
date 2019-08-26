// SPDX-License-Identifier: Unlicense OR MIT

/*
Package input exposes a unified interface for receiving input
events.

For example:

	var queue input.Queue = ...

	for e, ok := queue.Next(h); ok; e, ok = queue.Next(h) {
		switch e.(type) {
			...
		}
	}

In general, handlers must be declared before events become
available. Other packages such as pointer and key provide
the means for declaring handlers for specific input types.

The following example marks a handler ready for key input:

	import gioui.org/ui/input
	import gioui.org/ui/key

	ops := new(ui.Ops)
	var h *Handler = ...
	key.InputOp{Key: h}.Add(ops)

*/
package input

// Queue maps an event handler key to the events
// available to the handler.
type Queue interface {
	// Next returns the next available event, or
	// false if none are available.
	Next(k Key) (Event, bool)
}

// Key is the stable identifier for an event handler.
// For a handler h, the key is typically &h.
type Key interface{}

// Event is the marker interface for events.
type Event interface {
	ImplementsEvent()
}
