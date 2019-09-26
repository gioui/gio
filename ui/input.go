// SPDX-License-Identifier: Unlicense OR MIT

package ui

// Queue maps an event handler key to the events
// available to the handler.
type Queue interface {
	// Events returns the available events for a
	// Key.
	Events(k Key) []Event
}

// Key is the stable identifier for an event handler.
// For a handler h, the key is typically &h.
type Key interface{}

// Event is the marker interface for events.
type Event interface {
	ImplementsEvent()
}
