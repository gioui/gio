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

// Event is sent when a key is pressed. For text input
// use EditEvent.
type Event struct {
	// Name is the rune character that most closely
	// match the key. For letters, the upper case form
	// is used.
	Name rune
	// Modifiers is the set of active modifiers when
	// the key was pressed.
	Modifiers Modifiers
}

// EditEvent is sent when text is input.
type EditEvent struct {
	Text string
}

// Modifiers
type Modifiers uint32

const (
	// ModCommand is the command modifier. On macOS
	// it is the Cmd key, on other platforms the Ctrl
	// key.
	ModCommand Modifiers = 1 << iota
	// THe shift key.
	ModShift
)

const (
	// Runes for special keys.
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

// Contain reports whether m contains all modifiers
// in m2.
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
