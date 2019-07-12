// SPDX-License-Identifier: Unlicense OR MIT

package pointer

import (
	"encoding/binary"
	"image"
	"time"

	"gioui.org/ui"
	"gioui.org/ui/f32"
	"gioui.org/ui/input"
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

type RectAreaOp struct {
	Size image.Point
}

type EllipseAreaOp struct {
	Size image.Point
}

// Must match the structure in input.areaOp
type areaOp struct {
	kind areaKind
	size image.Point
}

type HandlerOp struct {
	Key  input.Key
	Grab bool
}

// PassOp change the current event pass-through
// setting.
type PassOp struct {
	Pass bool
}

type ID uint16
type Type uint8
type Priority uint8
type Source uint8

// Must match input.areaKind
type areaKind uint8

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

const (
	areaRect areaKind = iota
	areaEllipse
)

func (op RectAreaOp) Add(ops *ui.Ops) {
	areaOp{
		kind: areaRect,
		size: op.Size,
	}.add(ops)
}

func (op EllipseAreaOp) Add(ops *ui.Ops) {
	areaOp{
		kind: areaEllipse,
		size: op.Size,
	}.add(ops)
}

func (op areaOp) add(o *ui.Ops) {
	data := make([]byte, ops.TypeAreaLen)
	data[0] = byte(ops.TypeArea)
	data[1] = byte(op.kind)
	bo := binary.LittleEndian
	bo.PutUint32(data[2:], uint32(op.size.X))
	bo.PutUint32(data[6:], uint32(op.size.Y))
	o.Write(data)
}

func (h HandlerOp) Add(o *ui.Ops) {
	data := make([]byte, ops.TypePointerHandlerLen)
	data[0] = byte(ops.TypePointerHandler)
	if h.Grab {
		data[1] = 1
	}
	o.Write(data, h.Key)
}

func (h *HandlerOp) Decode(d []byte, refs []interface{}) {
	if ops.OpType(d[0]) != ops.TypePointerHandler {
		panic("invalid op")
	}
	*h = HandlerOp{
		Grab: d[1] != 0,
		Key:  refs[0].(input.Key),
	}
}

func (op PassOp) Add(o *ui.Ops) {
	data := make([]byte, ops.TypePassLen)
	data[0] = byte(ops.TypePass)
	if op.Pass {
		data[1] = 1
	}
	o.Write(data)
}

func (op *PassOp) Decode(d []byte) {
	if ops.OpType(d[0]) != ops.TypePass {
		panic("invalid op")
	}
	*op = PassOp{
		Pass: d[1] != 0,
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

func (Event) ImplementsEvent()      {}
func (Event) ImplementsInputEvent() {}
