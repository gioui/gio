// SPDX-License-Identifier: Unlicense OR MIT

package clipboard

import (
	"io"

	"gioui.org/io/event"
)

// WriteCmd copies Text to the clipboard.
type WriteCmd struct {
	Type string
	Data io.ReadCloser
}

// ReadCmd requests the text of the clipboard, delivered to
// the handler through an [io/transfer.DataEvent].
type ReadCmd struct {
	Tag event.Tag
}

func (WriteCmd) ImplementsCommand() {}
func (ReadCmd) ImplementsCommand()  {}
