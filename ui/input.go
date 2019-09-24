// SPDX-License-Identifier: Unlicense OR MIT

package ui

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
