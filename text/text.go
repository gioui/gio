// SPDX-License-Identifier: Unlicense OR MIT

package text

import (
	"fmt"

	"gioui.org/io/system"
	"github.com/go-text/typesetting/font"
	"golang.org/x/image/math/fixed"
)

// Style is the font style.
type Style int

// Weight is a font weight, in CSS units subtracted 400 so the zero value
// is normal text weight.
type Weight int

// Font specify a particular typeface variant, style and weight.
type Font struct {
	Typeface Typeface
	Variant  Variant
	Style    Style
	// Weight is the text weight. If zero, Normal is used instead.
	Weight Weight
}

// Face is an opaque handle to a typeface. The concrete implementation depends
// upon the kind of font and shaper in use.
type Face interface {
	Face() font.Face
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
	Thin       Weight = -300
	ExtraLight Weight = -200
	Light      Weight = -100
	Normal     Weight = 0
	Medium     Weight = 100
	SemiBold   Weight = 200
	Bold       Weight = 300
	ExtraBold  Weight = 400
	Black      Weight = 500

	Hairline   = Thin
	UltraLight = ExtraLight
	DemiBold   = SemiBold
	UltraBold  = ExtraBold
	Heavy      = Black
	ExtraBlack = Black + 50
	UltraBlack = ExtraBlack
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
		panic("invalid Alignment")
	}
}

// Align returns the x offset that should be applied to text with width so that it
// appears correctly aligned within a space of size maxWidth and with the primary
// text direction dir.
func (a Alignment) Align(dir system.TextDirection, width fixed.Int26_6, maxWidth int) fixed.Int26_6 {
	mw := fixed.I(maxWidth)
	if dir.Progression() == system.TowardOrigin {
		switch a {
		case Start:
			a = End
		case End:
			a = Start
		}
	}
	switch a {
	case Middle:
		return (mw - width) / 2
	case End:
		return (mw - width)
	case Start:
		return 0
	default:
		panic(fmt.Errorf("unknown alignment %v", a))
	}
}

func (s Style) String() string {
	switch s {
	case Regular:
		return "Regular"
	case Italic:
		return "Italic"
	default:
		panic("invalid Style")
	}
}

func (w Weight) String() string {
	switch w {
	case Thin:
		return "Thin"
	case ExtraLight:
		return "ExtraLight"
	case Light:
		return "Light"
	case Normal:
		return "Normal"
	case Medium:
		return "Medium"
	case SemiBold:
		return "SemiBold"
	case Bold:
		return "Bold"
	case ExtraBold:
		return "ExtraBold"
	case Black:
		return "Black"
	case ExtraBlack:
		return "ExtraBlack"
	default:
		panic("invalid Weight")
	}
}
