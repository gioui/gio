// SPDX-License-Identifier: Unlicense OR MIT

package pointer

import (
	"encoding/binary"
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
	bo := binary.LittleEndian
	if h.Grab {
		data[1] = 1
	}
	bo.PutUint32(data[2:], uint32(o.Ref(h.Key)))
	bo.PutUint32(data[6:], uint32(o.Ref(h.Area)))
	o.Write(data)
}

func (h *OpHandler) Decode(d []byte, refs []interface{}) {
	bo := binary.LittleEndian
	if ops.OpType(d[0]) != ops.TypePointerHandler {
		panic("invalid op")
	}
	key := int(bo.Uint32(d[2:]))
	area := int(bo.Uint32(d[6:]))
	*h = OpHandler{
		Grab: d[1] != 0,
		Key:  refs[key].(Key),
		Area: refs[area].(Area),
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
