// SPDX-License-Identifier: Unlicense OR MIT

package key

import (
	"gioui.org/ui"
	"gioui.org/ui/internal/ops"
)

type OpHandler struct {
	Key   Key
	Focus bool
}

type OpHideInput struct{}

type Key interface{}

type Focus struct {
	Focus bool
}

type Chord struct {
	Name      rune
	Modifiers Modifiers
}

type Edit struct {
	Text string
}

type Modifiers uint32

type TextInputState uint8

const (
	ModCommand Modifiers = 1 << iota
	ModShift
)

const (
	TextInputKeep TextInputState = iota
	TextInputFocus
	TextInputClosed
	TextInputOpen
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

func (h OpHandler) Add(o *ui.Ops) {
	data := make([]byte, ops.TypeKeyHandlerLen)
	data[0] = byte(ops.TypeKeyHandler)
	if h.Focus {
		data[1] = 1
	}
	o.Write(data, h.Key)
}

func (h *OpHandler) Decode(d []byte, refs []interface{}) {
	if ops.OpType(d[0]) != ops.TypeKeyHandler {
		panic("invalid op")
	}
	*h = OpHandler{
		Focus: d[1] != 0,
		Key:   refs[0].(Key),
	}
}

func (h OpHideInput) Add(o *ui.Ops) {
	data := make([]byte, ops.TypeHideInputLen)
	data[0] = byte(ops.TypeHideInput)
	o.Write(data)
}

func (Edit) ImplementsEvent()       {}
func (Chord) ImplementsEvent()      {}
func (Focus) ImplementsEvent()      {}
func (Edit) ImplementsInputEvent()  {}
func (Chord) ImplementsInputEvent() {}
func (Focus) ImplementsInputEvent() {}
