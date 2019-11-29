// SPDX-License-Identifier: Unlicense OR MIT

// Package window implements platform specific windows
// and GPU contexts.
package window

import (
	"errors"
	"math"
	"time"

	"gioui.org/app/internal/gl"
	"gioui.org/io/event"
	"gioui.org/io/system"
	"gioui.org/unit"
)

type Options struct {
	Width, Height unit.Value
	Title         string
}

type FrameEvent struct {
	system.FrameEvent

	Sync bool
}

type Callbacks interface {
	SetDriver(d Driver)
	Event(e event.Event)
}

type Context interface {
	Functions() *gl.Functions
	Present() error
	MakeCurrent() error
	Release()
	Lock()
	Unlock()
}

// Driver is the interface for the platform implementation
// of a window.
type Driver interface {
	// SetAnimating sets the animation flag. When the window is animating,
	// FrameEvents are delivered as fast as the display can handle them.
	SetAnimating(anim bool)
	// ShowTextInput updates the virtual keyboard state.
	ShowTextInput(show bool)
	NewContext() (Context, error)
}

type windowRendezvous struct {
	in   chan windowAndOptions
	out  chan windowAndOptions
	errs chan error
}

type windowAndOptions struct {
	window Callbacks
	opts   *Options
}

// config implements the system.Config interface.
type config struct {
	// Device pixels per dp.
	pxPerDp float32
	// Device pixels per sp.
	pxPerSp float32
	now     time.Time
}

func (c *config) Now() time.Time {
	return c.now
}

func (c *config) Px(v unit.Value) int {
	var r float32
	switch v.U {
	case unit.UnitPx:
		r = v.V
	case unit.UnitDp:
		r = c.pxPerDp * v.V
	case unit.UnitSp:
		r = c.pxPerSp * v.V
	default:
		panic("unknown unit")
	}
	return int(math.Round(float64(r)))
}

func newWindowRendezvous() *windowRendezvous {
	wr := &windowRendezvous{
		in:   make(chan windowAndOptions),
		out:  make(chan windowAndOptions),
		errs: make(chan error),
	}
	go func() {
		var main windowAndOptions
		var out chan windowAndOptions
		for {
			select {
			case w := <-wr.in:
				var err error
				if main.window != nil {
					err = errors.New("multiple windows are not supported")
				}
				wr.errs <- err
				main = w
				out = wr.out
			case out <- main:
			}
		}
	}()
	return wr
}
