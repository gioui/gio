// SPDX-License-Identifier: Unlicense OR MIT

package pointer

import (
	"encoding/binary"
	"image"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/internal/opconst"
	"gioui.org/io/event"
	"gioui.org/io/key"
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
	// Buttons are the set of pressed mouse buttons for this event.
	Buttons Buttons
	// Position is the position of the event, relative to
	// the current transformation, as set by op.TransformOp.
	Position f32.Point
	// Scroll is the scroll amount, if any.
	Scroll f32.Point
	// Scale is the rate of a pinch gesture
	Magnification float32
	// Rotation is the rate of a rotate gesture
	Rotation float32
	// Modifiers is the set of active modifiers when
	// the mouse button was pressed.
	Modifiers key.Modifiers
}

// AreaOp updates the hit area to the intersection of the current
// hit area and the area. The area is transformed before applying
// it.
type AreaOp struct {
	kind areaKind
	rect image.Rectangle
}

// CursorNameOp sets the cursor for the current area.
type CursorNameOp struct {
	Name CursorName
}

// InputOp declares an input handler ready for pointer
// events.
type InputOp struct {
	Tag event.Tag
	// Grab, if set, request that the handler get
	// Grabbed priority.
	Grab bool
	// Types is a bitwise-or of event types to receive.
	Types Type
}

// PassOp sets the pass-through mode.
type PassOp struct {
	Pass bool
}

type ID uint16

// Type of an Event.
type Type uint16

// Priority of an Event.
type Priority uint8

// Source of an Event.
type Source uint8

// Buttons is a set of mouse buttons
type Buttons uint8

// CursorName is the name of a cursor.
type CursorName string

// Must match app/internal/input.areaKind
type areaKind uint8

const (
	// CursorDefault is the default cursor.
	CursorDefault CursorName = ""
	// CursorText is the cursor for text.
	CursorText CursorName = "text"
	// CursorPointer is the cursor for a link.
	CursorPointer CursorName = "pointer"
	// CursorCrossHair is the cursor for precise location.
	CursorCrossHair CursorName = "crosshair"
	// CursorColResize is the cursor for vertical resize.
	CursorColResize CursorName = "col-resize"
	// CursorRowResize is the cursor for horizontal resize.
	CursorRowResize CursorName = "row-resize"
	// CursorGrab is the cursor for moving object in any direction.
	CursorGrab CursorName = "grab"
	// CursorNone hides the cursor. To show it again, use any other cursor.
	CursorNone CursorName = "none"
)

const (
	// A Cancel event is generated when the current gesture is
	// interrupted by other handlers or the system.
	Cancel Type = (1 << iota) >> 1
	// Press of a pointer.
	Press
	// Release of a pointer.
	Release
	// Move of a pointer.
	Move
	// Drag of a pointer.
	Drag
	// Pointer enters an area watching for pointer input
	Enter
	// Pointer leaves an area watching for pointer input
	Leave
	// Scroll of a pointer.
	Scroll
	// Pinch gesture
	Pinch
	// Rotate gesture
	Rotate
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
	// Foremost priority is like Shared, but the
	// handler is the foremost of the matching set.
	Foremost
	// Grabbed is used for matching sets of size 1.
	Grabbed
)

const (
	ButtonLeft Buttons = 1 << iota
	ButtonRight
	ButtonMiddle
)

const (
	areaRect areaKind = iota
	areaEllipse
)

// Rect constructs a rectangular hit area.
func Rect(size image.Rectangle) AreaOp {
	return AreaOp{
		kind: areaRect,
		rect: size,
	}
}

// Ellipse constructs an ellipsoid hit area.
func Ellipse(size image.Rectangle) AreaOp {
	return AreaOp{
		kind: areaEllipse,
		rect: size,
	}
}

func (op AreaOp) Add(o *op.Ops) {
	data := o.Write(opconst.TypeAreaLen)
	data[0] = byte(opconst.TypeArea)
	data[1] = byte(op.kind)
	bo := binary.LittleEndian
	bo.PutUint32(data[2:], uint32(op.rect.Min.X))
	bo.PutUint32(data[6:], uint32(op.rect.Min.Y))
	bo.PutUint32(data[10:], uint32(op.rect.Max.X))
	bo.PutUint32(data[14:], uint32(op.rect.Max.Y))
}

func (op CursorNameOp) Add(o *op.Ops) {
	data := o.Write1(opconst.TypeCursorLen, op.Name)
	data[0] = byte(opconst.TypeCursor)
}

func (h InputOp) Add(o *op.Ops) {
	if h.Tag == nil {
		panic("Tag must be non-nil")
	}
	data := o.Write1(opconst.TypePointerInputLen, h.Tag)
	data[0] = byte(opconst.TypePointerInput)
	if h.Grab {
		data[1] = 1
	}
	data[2] = byte(h.Types)
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
	case Drag:
		return "Drag"
	case Enter:
		return "Enter"
	case Leave:
		return "Leave"
	case Scroll:
		return "Scroll"
	case Pinch:
		return "Pinch"
	case Rotate:
		return "Rotate"
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

// Contain reports whether the set b contains
// all of the buttons.
func (b Buttons) Contain(buttons Buttons) bool {
	return b&buttons == buttons
}

func (b Buttons) String() string {
	var strs []string
	if b.Contain(ButtonLeft) {
		strs = append(strs, "ButtonLeft")
	}
	if b.Contain(ButtonRight) {
		strs = append(strs, "ButtonRight")
	}
	if b.Contain(ButtonMiddle) {
		strs = append(strs, "ButtonMiddle")
	}
	return strings.Join(strs, "|")
}

func (c CursorName) String() string {
	if c == CursorDefault {
		return "default"
	}
	return string(c)
}

func (Event) ImplementsEvent() {}
