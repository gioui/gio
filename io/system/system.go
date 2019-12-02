// SPDX-License-Identifier: Unlicense OR MIT

// Package system contains events usually handled at the top-level
// program level.
package system

import (
	"image"
	"time"

	"gioui.org/op"
	"gioui.org/unit"
)

// A FrameEvent asks for a new frame in the form of a list of
// operations.
type FrameEvent struct {
	Config Config
	// Size is the dimensions of the window.
	Size image.Point
	// Insets is the insets to apply.
	Insets Insets
	// Frame replaces the window's frame with the new
	// frame.
	Frame func(frame *op.Ops)
	// Whether this draw is system generated and needs a complete
	// frame before proceeding.
	sync bool
}

// Config defines the essential properties of
// the environment.
type Config interface {
	// Now returns the current animation time.
	Now() time.Time

	unit.Converter
}

// DestroyEvent is the last event sent through
// a window event channel.
type DestroyEvent struct {
	// Err is nil for normal window closures. If a
	// window is prematurely closed, Err is the cause.
	Err error
}

// Insets is the space taken up by
// system decoration such as translucent
// system bars and software keyboards.
type Insets struct {
	Top, Bottom, Left, Right unit.Value
}

// A StageEvent is generated whenever the stage of a
// Window changes.
type StageEvent struct {
	Stage Stage
}

// CommandEvent is a system event.
type CommandEvent struct {
	Type CommandType
	// Suppress the default action of the command.
	Cancel bool
}

// Stage of a Window.
type Stage uint8

// CommandType is the type of a CommandEvent.
type CommandType uint8

const (
	// StagePaused is the Stage for inactive Windows.
	// Inactive Windows don't receive FrameEvents.
	StagePaused Stage = iota
	// StateRunning is for active Windows.
	StageRunning
)

const (
	// CommandBack is the command for a back action
	// such as the Android back button.
	CommandBack CommandType = iota
)

func (l Stage) String() string {
	switch l {
	case StagePaused:
		return "StagePaused"
	case StageRunning:
		return "StageRunning"
	default:
		panic("unexpected Stage value")
	}
}

func (_ FrameEvent) ImplementsEvent()    {}
func (_ StageEvent) ImplementsEvent()    {}
func (_ *CommandEvent) ImplementsEvent() {}
func (_ DestroyEvent) ImplementsEvent()  {}
