// SPDX-License-Identifier: Unlicense OR MIT

package pointer

import (
	"time"

	"gioui.org/ui"
	"gioui.org/ui/f32"
	"gioui.org/ui/internal/ops"
)

type Event struct {
	Type      Type
	Source    Source
	PointerID ID
	Priority  Priority
	Time      time.Duration
	Hit       bool
	Position  f32.Point
	Scroll    f32.Point
}

type OpHandler struct {
	Key  Key
	Area Area
	Grab bool
}

type Area interface {
	Hit(pos f32.Point) HitResult
}

type Key interface{}

type Events interface {
	For(k Key) []Event
}

type HitResult uint8

const (
	HitNone HitResult = iota
	HitTransparent
	HitOpaque
)

type ID uint16
type Type uint8
type Priority uint8
type Source uint8

const (
	Cancel Type = iota
	Press
	Release
	Move
)

const (
	Mouse Source = iota
	Touch
)

const (
	Shared Priority = iota
	Foremost
	Grabbed
)

func (h OpHandler) Add(o *ui.Ops) {
	data := make([]byte, ops.TypePointerHandlerLen)
	data[0] = byte(ops.TypePointerHandler)
	if h.Grab {
		data[1] = 1
	}
	o.Write(data, []interface{}{h.Key, h.Area})
}

func (h *OpHandler) Decode(d []byte, refs []interface{}) {
	if ops.OpType(d[0]) != ops.TypePointerHandler {
		panic("invalid op")
	}
	*h = OpHandler{
		Grab: d[1] != 0,
		Key:  refs[0].(Key),
		Area: refs[1].(Area),
	}
}

func (t Type) String() string {
	switch t {
	case Press:
		return "Press"
	case Release:
		return "Release"
	case Cancel:
		return "Cancel"
	case Move:
		return "Move"
	default:
		panic("unknown Type")
	}
}

func (p Priority) String() string {
	switch p {
	case Shared:
		return "Shared"
	case Foremost:
		return "Foremost"
	case Grabbed:
		return "Grabbed"
	default:
		panic("unknown priority")
	}
}

func (s Source) String() string {
	switch s {
	case Mouse:
		return "Mouse"
	case Touch:
		return "Touch"
	default:
		panic("unknown source")
	}
}

func (Event) ImplementsEvent() {}
