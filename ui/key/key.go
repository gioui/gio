// SPDX-License-Identifier: Unlicense OR MIT

package key

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

func (OpHandler) ImplementsOp()   {}
func (OpHideInput) ImplementsOp() {}

func (Edit) ImplementsEvent()  {}
func (Chord) ImplementsEvent() {}
func (Focus) ImplementsEvent() {}
func (Edit) isKeyEvent()       {}
func (Chord) isKeyEvent()      {}
func (Focus) isKeyEvent()      {}
