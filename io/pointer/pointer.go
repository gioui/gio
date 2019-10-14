// SPDX-License-Identifier: Unlicense OR MIT

package pointer

import (
	"encoding/binary"
	"image"
	"time"

	"gioui.org/f32"
	"gioui.org/internal/opconst"
	"gioui.org/io/event"
	"gioui.org/op"
)

// Event is a pointer event.
type Event struct {
	Type   Type
	Source Source
	// PointerID is the id for the pointer and can be used
	// to track a particular pointer from Press to
	// Release or Cancel.
	PointerID ID
	// Priority is the priority of the receiving handler
	// for this event.
	Priority Priority
	// Time is when the event was received. The
	// timestamp is relative to an undefined base.
	Time time.Duration
	// Hit is set when the event was within the registered
	// area for the handler. Hit can be false when a pointer
	// was pressed within the hit area, and then dragged
	// outside it.
	Hit bool
	// Position is the position of the event, relative to
	// the current transformation, as set by op.TransformOp.
	Position f32.Point
	// Scroll is the scroll amount, if any.
	Scroll f32.Point
}

// RectAreaOp updates the hit area to the intersection
// of the current hit area with a rectangular area.
type RectAreaOp struct {
	// Rect defines the rectangle. The current transform
	// is applied to it.
	Rect image.Rectangle
}

// EllipseAreaOp updates the hit area to the intersection
// of the current hit area with an elliptical area.
type EllipseAreaOp struct {
	// Rect is the bounds for the ellipse. The current transform
	// is applied to the rectangle.
	Rect image.Rectangle
}

// Must match the structure in input.areaOp
type areaOp struct {
	kind areaKind
	rect image.Rectangle
}

// InputOp declares an input handler ready for pointer
// events.
type InputOp struct {
	Key event.Key
	// Grab, if set, request that the handler get
	// Grabbed priority.
	Grab bool
}

// PassOp sets the pass-through mode.
type PassOp struct {
	Pass bool
}

type ID uint16

// Type of an Event.
type Type uint8

// Priority of an Event.
type Priority uint8

// Source of an Event.
type Source uint8

// Must match app/internal/input.areaKind
type areaKind uint8

const (
	// A Cancel event is generated when the current gesture is
	// interrupted by other handlers or the system.
	Cancel Type = iota
	// Press of a pointer.
	Press
	// Release of a pointer.
	Release
	// Move of a pointer.
	Move
)

const (
	// Mouse generated event.
	Mouse Source = iota
	// Touch generated event.
	Touch
)

const (
	// Shared priority is for handlers that
	// are part of a matching set larger than 1.
	Shared Priority = iota
	// Grabbed is used for matching sets of size 1.
	Grabbed
)

const (
	areaRect areaKind = iota
	areaEllipse
)

func (op RectAreaOp) Add(ops *op.Ops) {
	areaOp{
		kind: areaRect,
		rect: op.Rect,
	}.add(ops)
}

func (op EllipseAreaOp) Add(ops *op.Ops) {
	areaOp{
		kind: areaEllipse,
		rect: op.Rect,
	}.add(ops)
}

func (op areaOp) add(o *op.Ops) {
	data := o.Write(opconst.TypeAreaLen)
	data[0] = byte(opconst.TypeArea)
	data[1] = byte(op.kind)
	bo := binary.LittleEndian
	bo.PutUint32(data[2:], uint32(op.rect.Min.X))
	bo.PutUint32(data[6:], uint32(op.rect.Min.Y))
	bo.PutUint32(data[10:], uint32(op.rect.Max.X))
	bo.PutUint32(data[14:], uint32(op.rect.Max.Y))
}

func (h InputOp) Add(o *op.Ops) {
	data := o.Write(opconst.TypePointerInputLen, h.Key)
	data[0] = byte(opconst.TypePointerInput)
	if h.Grab {
		data[1] = 1
	}
}

func (op PassOp) Add(o *op.Ops) {
	data := o.Write(opconst.TypePassLen)
	data[0] = byte(opconst.TypePass)
	if op.Pass {
		data[1] = 1
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
