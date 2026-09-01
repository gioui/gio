// SPDX-License-Identifier: Unlicense OR MIT

package app

// ClosingEvent is sent when the user requests to close a window.
// Call Abort while handling the event to keep the window open.
type ClosingEvent struct {
	abort bool
}

// Abort keeps the window open for this close request.
func (e *ClosingEvent) Abort() {
	e.abort = true
}

func (*ClosingEvent) ImplementsEvent() {}

// DestroyEvent is the last event sent through
// a window event channel.
type DestroyEvent struct {
	// Err is nil for normal window closures. If a
	// window is prematurely closed, Err is the cause.
	Err error
}

func (DestroyEvent) ImplementsEvent() {}
