// SPDX-License-Identifier: Unlicense OR MIT

// Package profiles provides access to rendering
// profiles.
package profile

import (
	"gioui.org/internal/opconst"
	"gioui.org/ui"
)

// Op registers a handler for receiving
// Events.
type Op struct {
	Key ui.Key
}

// Event contains profile data from a single
// rendered frame.
type Event struct {
	// Timings. Very likely to change.
	Timings string
}

func (p Op) Add(o *ui.Ops) {
	data := make([]byte, opconst.TypeProfileLen)
	data[0] = byte(opconst.TypeProfile)
	o.Write(data, p.Key)
}

func (p Event) ImplementsEvent() {}
