// SPDX-License-Identifier: Unlicense OR MIT

package text

import (
	"gioui.org/op"
	"gioui.org/unit"
	"golang.org/x/image/font"
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
	// MaxWidth is the available width of the layout.
	MaxWidth int
}

// Style is the font style.
type Style int

// Weight is a font weight, in CSS units.
type Weight int

// Font specify a particular typeface, style and size.
type Font struct {
	Typeface Typeface
	Variant  Variant
	Size     unit.Value
	Style    Style
	// Weight is the text weight. If zero, Normal is used instead.
	Weight Weight
}

// Face implements text layout and shaping for a particular font.
type Face interface {
	Layout(ppem fixed.Int26_6, str string, opts LayoutOptions) *Layout
	Shape(ppem fixed.Int26_6, str String) op.CallOp
	Metrics(ppem fixed.Int26_6) font.Metrics
}

// Typeface identifies a particular typeface design. The empty
// string denotes the default typeface.
type Typeface string

// Variant denotes a typeface variant such as "Mono" or "Smallcaps".
type Variant string

type Alignment uint8

const (
	Start Alignment = iota
	End
	Middle
)

const (
	Regular Style = iota
	Italic
)

const (
	Normal Weight = 400
	Medium Weight = 500
	Bold   Weight = 600
)

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
