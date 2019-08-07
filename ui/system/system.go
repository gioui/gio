// SPDX-License-Identifier: Unlicense OR MIT

// Package system contain ops and types for
// system events.
package system

import (
	"gioui.org/ui"
	"gioui.org/ui/input"
	"gioui.org/ui/internal/opconst"
)

// ProfileOp registers a handler for receiving
// ProfileEvents.
type ProfileOp struct {
	Key input.Key
}

// ProfileEvent contain profile data from a single
// rendered frame.
type ProfileEvent struct {
	// String with timings. Very likely to change.
	Timings string
}

func (p ProfileOp) Add(o *ui.Ops) {
	data := make([]byte, opconst.TypeProfileLen)
	data[0] = byte(opconst.TypeProfile)
	o.Write(data, p.Key)
}

func (p *ProfileOp) Decode(d []byte, refs []interface{}) {
	if opconst.OpType(d[0]) != opconst.TypeProfile {
		panic("invalid op")
	}
	*p = ProfileOp{
		Key: refs[0].(input.Key),
	}
}

func (p ProfileEvent) ImplementsEvent() {}
