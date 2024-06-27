// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/io/input"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// extraArgs contains extra arguments to append to
// os.Args. The arguments are separated with |.
// Useful for running programs on mobiles where the
// command line is not available.
// Set with the go linker flag -X.
var extraArgs string

// ID is the app id exposed to the platform.
//
// On Android ID is the package property of AndroidManifest.xml,
// on iOS ID is the CFBundleIdentifier of the app Info.plist,
// on Wayland it is the toplevel app_id,
// on X11 it is the X11 XClassHint.
//
// ID is set by the [gioui.org/cmd/gogio] tool or manually with the -X linker flag. For example,
//
//	go build -ldflags="-X 'gioui.org/app.ID=org.gioui.example.Kitchen'" .
//
// Note that ID is treated as a constant, and that changing it at runtime
// is not supported. The default value of ID is filepath.Base(os.Args[0]).
var ID = ""

// A FrameEvent requests a new frame in the form of a list of
// operations that describes the window content.
type FrameEvent struct {
	// Now is the current animation. Use Now instead of time.Now to
	// synchronize animation and to avoid the time.Now call overhead.
	Now time.Time
	// Metric converts device independent dp and sp to device pixels.
	Metric unit.Metric
	// Size is the dimensions of the window.
	Size image.Point
	// Insets represent the space occupied by system decorations and controls.
	Insets Insets
	// Frame completes the FrameEvent by drawing the graphical operations
	// from ops into the window.
	Frame func(frame *op.Ops)
	// Source is the interface between the window and widgets.
	Source input.Source
}

// ViewEvent provides handles to the underlying window objects for the
// current display protocol.
type ViewEvent interface {
	implementsViewEvent()
	ImplementsEvent()
	// Valid will return true when the ViewEvent does contains valid handles.
	// If a window receives an invalid ViewEvent, it should deinitialize any
	// state referring to handles from a previous ViewEvent.
	Valid() bool
}

// Insets is the space taken up by
// system decoration such as translucent
// system bars and software keyboards.
type Insets struct {
	// Values are in pixels.
	Top, Bottom, Left, Right unit.Dp
}

// NewContext is shorthand for
//
//	layout.Context{
//	  Ops: ops,
//	  Now: e.Now,
//	  Source: e.Source,
//	  Metric: e.Metric,
//	  Constraints: layout.Exact(e.Size),
//	}
//
// NewContext calls ops.Reset and adjusts ops for e.Insets.
func NewContext(ops *op.Ops, e FrameEvent) layout.Context {
	ops.Reset()

	size := e.Size

	if e.Insets != (Insets{}) {
		left := e.Metric.Dp(e.Insets.Left)
		top := e.Metric.Dp(e.Insets.Top)
		op.Offset(image.Point{
			X: left,
			Y: top,
		}).Add(ops)

		size.X -= left + e.Metric.Dp(e.Insets.Right)
		size.Y -= top + e.Metric.Dp(e.Insets.Bottom)
	}

	return layout.Context{
		Ops:         ops,
		Now:         e.Now,
		Source:      e.Source,
		Metric:      e.Metric,
		Constraints: layout.Exact(size),
	}
}

// DataDir returns a path to use for application-specific
// configuration data.
// On desktop systems, DataDir use os.UserConfigDir.
// On iOS NSDocumentDirectory is queried.
// For Android Context.getFilesDir is used.
//
// BUG: DataDir blocks on Android until init functions
// have completed.
func DataDir() (string, error) {
	return dataDir()
}

// Main must be called last from the program main function.
// On most platforms Main blocks forever, for Android and
// iOS it returns immediately to give control of the main
// thread back to the system.
//
// Calling Main is necessary because some operating systems
// require control of the main thread of the program for
// running windows.
func Main() {
	osMain()
}

func (FrameEvent) ImplementsEvent() {}

func init() {
	if extraArgs != "" {
		args := strings.Split(extraArgs, "|")
		os.Args = append(os.Args, args...)
	}
	if ID == "" {
		ID = filepath.Base(os.Args[0])
	}
}
