// SPDX-License-Identifier: Unlicense OR MIT

// package app implements platform specific windows
// and GPU contexts.
package app

import (
	"errors"
	"image/color"

	"gioui.org/io/key"

	"gioui.org/gpu"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/unit"
)

type size struct {
	Width  unit.Value
	Height unit.Value
}

type config struct {
	Size            *size
	MinSize         *size
	MaxSize         *size
	Title           *string
	WindowMode      *windowMode
	StatusColor     *color.NRGBA
	NavigationColor *color.NRGBA
	Orientation     *orientation
	CustomRenderer  bool
}

type wakeupEvent struct{}

type windowMode uint8

const (
	windowed windowMode = iota
	fullscreen
)

type orientation uint8

const (
	anyOrientation orientation = iota
	landscapeOrientation
	portraitOrientation
)

type frameEvent struct {
	system.FrameEvent

	Sync bool
}

type context interface {
	API() gpu.API
	RenderTarget() gpu.RenderTarget
	Present() error
	Refresh() error
	Release()
	Lock() error
	Unlock()
}

// errDeviceLost is returned from Context.Present when
// the underlying GPU device is gone and should be
// recreated.
var errDeviceLost = errors.New("GPU device lost")

// Driver is the interface for the platform implementation
// of a window.
type driver interface {
	// SetAnimating sets the animation flag. When the window is animating,
	// FrameEvents are delivered as fast as the display can handle them.
	SetAnimating(anim bool)

	// ShowTextInput updates the virtual keyboard state.
	ShowTextInput(show bool)

	SetInputHint(mode key.InputHint)

	NewContext() (context, error)

	// ReadClipboard requests the clipboard content.
	ReadClipboard()
	// WriteClipboard requests a clipboard write.
	WriteClipboard(s string)

	// Configure the window.
	Configure(cnf *config)

	// SetCursor updates the current cursor to name.
	SetCursor(name pointer.CursorName)

	// Close the window.
	Close()
	// Wakeup wakes up the event loop and sends a WakeupEvent.
	Wakeup()
}

type windowRendezvous struct {
	in   chan windowAndConfig
	out  chan windowAndConfig
	errs chan error
}

type windowAndConfig struct {
	window *callbacks
	cnf    *config
}

func newWindowRendezvous() *windowRendezvous {
	wr := &windowRendezvous{
		in:   make(chan windowAndConfig),
		out:  make(chan windowAndConfig),
		errs: make(chan error),
	}
	go func() {
		var main windowAndConfig
		var out chan windowAndConfig
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

func (_ wakeupEvent) ImplementsEvent() {}
