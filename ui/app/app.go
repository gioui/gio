// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"errors"
	"image"
	"math"
	"os"
	"strings"
	"time"

	"gioui.org/ui"
)

// UpdateEvent is sent when a Window's Update
// method must be called.
type UpdateEvent struct {
	Config Config
	// Size is the dimensions of the window.
	Size image.Point
	// Insets is the insets to apply.
	Insets Insets
	// Whether this draw is system generated
	// and needs a complete frame before
	// proceeding.
	sync bool
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
	Top, Bottom, Left, Right ui.Value
}

// StageEvent is sent whenever the stage of a
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

type windowRendezvous struct {
	in   chan windowAndOptions
	out  chan windowAndOptions
	errs chan error
}

type windowAndOptions struct {
	window *Window
	opts   *WindowOptions
}

const (
	// StagePaused is the Stage for inactive Windows.
	// Inactive Windows don't receive UpdateEvents.
	StagePaused Stage = iota
	// StateRunning is for active Windows.
	StageRunning
)

const (
	// CommandBack is the command for a back action
	// such as the Android back button.
	CommandBack CommandType = iota
)

const (
	inchPrDp = 1.0 / 160
	mmPrDp   = 25.4 / 160
	// monitorScale is the extra scale applied to
	// monitor outputs to compensate for the extra
	// viewing distance compared to phone and tables.
	monitorScale = 1.20
	// minDensity is the minimum pixels per dp to
	// ensure font and ui legibility on low-dpi
	// screens.
	minDensity = 1.25
)

// extraArgs contains extra arguments to append to
// os.Args. The arguments are separated with |.
// Useful for running programs on mobiles where the
// command line is not available.
// Set with the go linker flag -X.
var extraArgs string

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

func init() {
	args := strings.Split(extraArgs, "|")
	os.Args = append(os.Args, args...)
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

// Main must be called from the a program's main function. It
// blocks until there are no more windows active.
//
// Calling Main is necessary because some operating systems
// require control of the main thread of the program for
// running windows.
func Main() {
	main()
}

// Config implements the ui.Config interface.
type Config struct {
	// Device pixels per dp.
	pxPerDp float32
	// Device pixels per sp.
	pxPerSp float32
	now     time.Time
}

func (c *Config) Now() time.Time {
	return c.now
}

func (c *Config) Px(v ui.Value) int {
	var r float32
	switch v.U {
	case ui.UnitPx:
		r = v.V
	case ui.UnitDp:
		r = c.pxPerDp * v.V
	case ui.UnitSp:
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

func (_ UpdateEvent) ImplementsEvent()   {}
func (_ StageEvent) ImplementsEvent()    {}
func (_ *CommandEvent) ImplementsEvent() {}
func (_ DestroyEvent) ImplementsEvent()  {}
