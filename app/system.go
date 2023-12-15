// SPDX-License-Identifier: Unlicense OR MIT

package app

// DestroyEvent is the last event sent through
// a window event channel.
type DestroyEvent struct {
	// Err is nil for normal window closures. If a
	// window is prematurely closed, Err is the cause.
	Err error
}

func (DestroyEvent) ImplementsEvent() {}
