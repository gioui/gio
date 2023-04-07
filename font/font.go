/*
Package font provides type describing font faces attributes.
*/
package font

import "github.com/go-text/typesetting/font"

// A FontFace is a Font and a matching Face.
type FontFace struct {
	Font Font
	Face Face
}

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
)

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
	default:
		panic("invalid Weight")
	}
}
