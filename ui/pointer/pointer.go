// SPDX-License-Identifier: Unlicense OR MIT

package pointer

import (
	"encoding/binary"
	"image"
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

type OpArea struct {
	Transparent bool

	kind areaKind
	size image.Point
}

type OpHandler struct {
	Key  Key
	Grab bool
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

func AreaRect(size image.Point) OpArea {
	return OpArea{
		kind: areaRect,
		size: size,
	}
}

func AreaEllipse(size image.Point) OpArea {
	return OpArea{
		kind: areaEllipse,
		size: size,
	}
}

func (op OpArea) Add(o *ui.Ops) {
	data := make([]byte, ops.TypeAreaLen)
	data[0] = byte(ops.TypeArea)
	data[1] = byte(op.kind)
	bo := binary.LittleEndian
	bo.PutUint32(data[2:], uint32(op.size.X))
	bo.PutUint32(data[6:], uint32(op.size.Y))
	o.Write(data)
}

func (op *OpArea) decode(d []byte) {
	if ops.OpType(d[0]) != ops.TypeArea {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	size := image.Point{
		X: int(bo.Uint32(d[2:])),
		Y: int(bo.Uint32(d[6:])),
	}
	*op = OpArea{
		kind: areaKind(d[1]),
		size: size,
	}
}

func (op *OpArea) hit(pos f32.Point) HitResult {
	res := HitOpaque
	if op.Transparent {
		res = HitTransparent
	}
	switch op.kind {
	case areaRect:
		if 0 <= pos.X && pos.X < float32(op.size.X) &&
			0 <= pos.Y && pos.Y < float32(op.size.Y) {
			return res
		} else {
			return HitNone
		}
	case areaEllipse:
		rx := float32(op.size.X) / 2
		ry := float32(op.size.Y) / 2
		rx2 := rx * rx
		ry2 := ry * ry
		xh := pos.X - rx
		yk := pos.Y - ry
		if xh*xh*ry2+yk*yk*rx2 <= rx2*ry2 {
			return res
		} else {
			return HitNone
		}
	default:
		panic("invalid area kind")
	}
}

func (h OpHandler) Add(o *ui.Ops) {
	data := make([]byte, ops.TypePointerHandlerLen)
	data[0] = byte(ops.TypePointerHandler)
	if h.Grab {
		data[1] = 1
	}
	o.Write(data, h.Key)
}

func (h *OpHandler) Decode(d []byte, refs []interface{}) {
	if ops.OpType(d[0]) != ops.TypePointerHandler {
		panic("invalid op")
	}
	*h = OpHandler{
		Grab: d[1] != 0,
		Key:  refs[0].(Key),
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
