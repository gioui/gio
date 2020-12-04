// SPDX-License-Identifier: Unlicense OR MIT

package clipboard

// Event is generated when the clipboard content is requested.
type Event struct {
	Text string
}

func (Event) ImplementsEvent() {}
