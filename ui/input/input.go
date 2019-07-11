// SPDX-License-Identifier: Unlicense OR MIT

// Package input exposes a unified interface to input sources. Subpackages
// such as pointer and key provide the interfaces for specific input types.
package input

// Queue maps an event handler key to the events
// available to the handler.
type Queue interface {
	Events(k Key) []Event
}

// Key is the stable identifier for an event handler. For a handler h, the
// key is typically &h.
type Key interface{}

// Event is the marker interface for input events.
type Event interface {
	ImplementsInputEvent()
}
