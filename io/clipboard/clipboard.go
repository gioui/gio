// SPDX-License-Identifier: Unlicense OR MIT

package clipboard

import (
	"io"

	"gioui.org/internal/ops"
	"gioui.org/io/event"
	"gioui.org/op"
)

// Event is generated when the clipboard content is requested.
type Event struct {
	Text string
}

// WriteCmd copies Text to the clipboard.
type WriteCmd struct {
	Type string
	Data io.ReadCloser
}

// ReadOp requests the text of the clipboard, delivered to
// the current handler through an Event.
type ReadOp struct {
	Tag event.Tag
}

func (h ReadOp) Add(o *op.Ops) {
	data := ops.Write1(&o.Internal, ops.TypeClipboardReadLen, h.Tag)
	data[0] = byte(ops.TypeClipboardRead)
}

func (Event) ImplementsEvent() {}

func (WriteCmd) ImplementsCommand() {}
