// SPDX-License-Identifier: Unlicense OR MIT

package text

import (
	"fmt"
	"image"

	"gioui.org/ui"
	"gioui.org/layout"
	"golang.org/x/image/math/fixed"
)

// A Line contains the measurements of a line of text.
type Line struct {
	Text String
	// Width is the width of the line.
	Width fixed.Int26_6
	// Ascent is the height above the baseline.
	Ascent fixed.Int26_6
	// Descent is the height below the baseline, including
	// the line gap.
	Descent fixed.Int26_6
	// Bounds is the visible bounds of the line.
	Bounds fixed.Rectangle26_6
}

type String struct {
	String string
	// Advances contain the advance of each rune in String.
	Advances []fixed.Int26_6
}

// A Layout contains the measurements of a body of text as
// a list of Lines.
type Layout struct {
	Lines []Line
}

// LayoutOptions specify the constraints of a text layout.
type LayoutOptions struct {
	// MaxWidth set the maximum width of the layout.
	MaxWidth int
	// SingleLine specify that line breaks are ignored.
	SingleLine bool
}

type Face interface {
	// Layout returns the text layout for a string given a set of
	// options.
	Layout(s string, opts LayoutOptions) *Layout
	// Path returns the ClipOp outline of a text recorded in a macro.
	Path(s String) ui.MacroOp
}

type Alignment uint8

const (
	Start Alignment = iota
	End
	Middle
)

func linesDimens(lines []Line) layout.Dimensions {
	var width fixed.Int26_6
	var h int
	var baseline int
	if len(lines) > 0 {
		baseline = lines[0].Ascent.Ceil()
		var prevDesc fixed.Int26_6
		for _, l := range lines {
			h += (prevDesc + l.Ascent).Ceil()
			prevDesc = l.Descent
			if l.Width > width {
				width = l.Width
			}
		}
		h += lines[len(lines)-1].Descent.Ceil()
	}
	w := width.Ceil()
	return layout.Dimensions{
		Size: image.Point{
			X: w,
			Y: h,
		},
		Baseline: baseline,
	}
}

func align(align Alignment, width fixed.Int26_6, maxWidth int) fixed.Int26_6 {
	mw := fixed.I(maxWidth)
	switch align {
	case Middle:
		return fixed.I(((mw - width) / 2).Floor())
	case End:
		return fixed.I((mw - width).Floor())
	case Start:
		return 0
	default:
		panic(fmt.Errorf("unknown alignment %v", align))
	}
}

func (a Alignment) String() string {
	switch a {
	case Start:
		return "Start"
	case End:
		return "End"
	case Middle:
		return "Middle"
	default:
		panic("unreachable")
	}
}
