// SPDX-License-Identifier: Unlicense OR MIT

// Package event contains types for event handling.
package event

// Tag is the stable identifier for an event handler.
// For a handler h, the tag is typically &h.
type Tag interface{}

// Event is the marker interface for events.
type Event interface {
	ImplementsEvent()
}
