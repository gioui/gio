// SPDX-License-Identifier: Unlicense OR MIT

package key

import (
	"encoding/binary"

	"gioui.org/ui"
	"gioui.org/ui/internal/ops"
)

type OpHandler struct {
	Key   Key
	Focus bool
}

type OpHideInput struct{}

type Key interface{}

type Events interface {
	For(k Key) []Event
}

type Event interface {
	isKeyEvent()
}

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
	bo := binary.LittleEndian
	if h.Focus {
		data[1] = 1
	}
	bo.PutUint32(data[2:], uint32(o.Ref(h.Key)))
	o.Write(data)
}

func (h *OpHandler) Decode(d []byte, refs []interface{}) {
	bo := binary.LittleEndian
	if ops.OpType(d[0]) != ops.TypeKeyHandler {
		panic("invalid op")
	}
	key := int(bo.Uint32(d[2:]))
	*h = OpHandler{
		Focus: d[1] != 0,
		Key:   refs[key].(Key),
	}
}

func (h OpHideInput) Add(o *ui.Ops) {
	data := make([]byte, ops.TypeHideInputLen)
	data[0] = byte(ops.TypeHideInput)
	o.Write(data)
}

func (Edit) ImplementsEvent()  {}
func (Chord) ImplementsEvent() {}
func (Focus) ImplementsEvent() {}
func (Edit) isKeyEvent()       {}
func (Chord) isKeyEvent()      {}
func (Focus) isKeyEvent()      {}
