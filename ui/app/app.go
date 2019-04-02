// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"image"
	"os"
	"strings"

	"gioui.org/ui"
)

type Draw struct {
	Config *ui.Config
	Size   image.Point
	// Whether this draw is system generated
	// and needs to a complete frame before
	// proceeding.
	sync bool
}

type ChangeStage struct {
	Stage Stage
}

type Stage uint8

type Event interface {
	ImplementsEvent()
}

type Input interface {
	ImplementsInput()
}

const (
	StageDead Stage = iota
	StageInvisible
	StageVisible
)

const (
	inchPrDp = 1.0 / 160
	mmPrDp   = 25.4 / 160
	// monitorScale is the extra scale applied to
	// monitor outputs to compensate for the extra
	// viewing distance compared to phone and tables.
	monitorScale = 1.50
	// minDensity is the minimum pixels per dp to
	// ensure font and ui legibility on low-dpi
	// screens.
	minDensity = 1.25
)

// extraArgs contains extra arguments to append to
// os.Args. The arguments are separated with |.
// Useful for running programs on mobiles where the
// command line is not available.
// Set it with the go tool linker flag -X.
var extraArgs string

var windows = make(chan *Window)

// CreateWindow creates a new window for a set of window
// options. The options are hints; the platform is free to
// ignore or adjust them.
// CreateWindow is not supported on iOS and Android.
func CreateWindow(opts *WindowOptions) error {
	if opts == nil {
		opts = &WindowOptions{
			Width:  ui.Dp(800),
			Height: ui.Dp(600),
			Title:  "Gio program",
		}
	}
	if opts.Width.V <= 0 || opts.Height.V <= 0 {
		panic("window width and height must be larger than 0")
	}
	return createWindow(opts)
}

func Windows() <-chan *Window {
	return windows
}

func (l Stage) String() string {
	switch l {
	case StageDead:
		return "StageDead"
	case StageInvisible:
		return "StageInvisible"
	case StageVisible:
		return "StageVisible"
	default:
		panic("unexpected Stage value")
	}
}

func (_ Draw) ImplementsEvent()        {}
func (_ ChangeStage) ImplementsEvent() {}

func init() {
	args := strings.Split(extraArgs, "|")
	os.Args = append(os.Args, args...)
}
