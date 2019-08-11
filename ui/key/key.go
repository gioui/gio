// SPDX-License-Identifier: Unlicense OR MIT

/*
Package key implements key and text input handling.

The HandlerOp operations is used for declaring key
input handlers. Use the Queue interface from package
input to receive events.
*/
package key

import (
	"gioui.org/ui"
	"gioui.org/ui/input"
	"gioui.org/ui/internal/opconst"
)

// HandlerOp declares a handler ready for key events.
// Key events are in general only delivered to the
// focused key handler. Set the Focus flag to request
// the focus.
type HandlerOp struct {
	Key   input.Key
	Focus bool
}

// HideInputOp request that any on screen text input
// be hidden.
type HideInputOp struct{}

// FocusEvent is sent when a handler gains or looses
// focus.
type FocusEvent struct {
	Focus bool
}

type Event struct {
	Name      rune
	Modifiers Modifiers
}

type EditEvent struct {
	Text string
}

type Modifiers uint32

const (
	ModCommand Modifiers = 1 << iota
	ModShift
)

const (
	NameLeftArrow      = '←'
	NameRightArrow     = '→'
	NameUpArrow        = '↑'
	NameDownArrow      = '↓'
	NameReturn         = '⏎'
	NameEnter          = '⌤'
	NameEscape         = '⎋'
	NameHome           = '⇱'
	NameEnd            = '⇲'
	NameDeleteBackward = '⌫'
	NameDeleteForward  = '⌦'
	NamePageUp         = '⇞'
	NamePageDown       = '⇟'
)

func (m Modifiers) Contain(m2 Modifiers) bool {
	return m&m2 == m2
}

func (h HandlerOp) Add(o *ui.Ops) {
	data := make([]byte, opconst.TypeKeyHandlerLen)
	data[0] = byte(opconst.TypeKeyHandler)
	if h.Focus {
		data[1] = 1
	}
	o.Write(data, h.Key)
}

func (h HideInputOp) Add(o *ui.Ops) {
	data := make([]byte, opconst.TypeHideInputLen)
	data[0] = byte(opconst.TypeHideInput)
	o.Write(data)
}

func (EditEvent) ImplementsEvent()  {}
func (Event) ImplementsEvent()      {}
func (FocusEvent) ImplementsEvent() {}
