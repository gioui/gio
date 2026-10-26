// SPDX-License-Identifier: Unlicense OR MIT

package app

// FileDropEvent is sent when the user drops files onto a Window.
//
// Paths contains absolute local paths; an empty Paths reports a drop whose
// contents could not be resolved to any openable file (for example a stale
// Finder "recent item" or a promise-only drag).  Applications decide which
// extensions to accept and how to report an empty drop.
//
// The event is delivered on the window's event loop, so it is safe to open
// documents or call Window methods while handling it.
type FileDropEvent struct {
	Paths []string
}

func (FileDropEvent) ImplementsEvent() {}
