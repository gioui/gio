// SPDX-License-Identifier: Unlicense OR MIT

package text

import (
	"fmt"
	"image"

	"gioui.org/ui/draw"
	"gioui.org/ui/layout"
	"golang.org/x/image/math/fixed"
)

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
	String   string
	Advances []fixed.Int26_6
}

type Layout struct {
	Lines []Line
}

type Face interface {
	Layout(str string, singleLine bool, maxWidth int) *Layout
	Path(str String) *draw.Path
}

type Alignment uint8

const (
	Start Alignment = iota
	End
	Center
)

func linesDimens(lines []Line) layout.Dimens {
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
	return layout.Dimens{
		Size: image.Point{
			X: w,
			Y: h,
		},
		Baseline: baseline,
	}
}

func IsNewline(r rune) bool {
	return r == '\n'
}

func align(align Alignment, width fixed.Int26_6, maxWidth int) fixed.Int26_6 {
	mw := fixed.I(maxWidth)
	switch align {
	case Center:
		return fixed.I(((mw - width) / 2).Floor())
	case End:
		return fixed.I((mw - width).Floor())
	case Start:
		return 0
	default:
		panic(fmt.Errorf("unknown alignment %v", align))
	}
}
