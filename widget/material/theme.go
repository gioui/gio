// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"image/color"

	"golang.org/x/exp/shiny/materialdesign/icons"

	"gioui.org/font"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
)

// Palette contains the minimal set of colors that a widget may need to
// draw itself.
type Palette struct {
	// Bg is the background color atop which content is currently being
	// drawn.
	Bg color.NRGBA

	// Fg is a color suitable for drawing on top of Bg.
	Fg color.NRGBA

	// ContrastBg is a color used to draw attention to active,
	// important, interactive widgets such as buttons.
	ContrastBg color.NRGBA

	// ContrastFg is a color suitable for content drawn on top of
	// ContrastBg.
	ContrastFg color.NRGBA
}

// Theme holds the general theme of an app or window. Different top-level
// windows should have different instances of Theme (with different Shapers;
// see the godoc for [text.Shaper]), though their other fields can be equal.
type Theme struct {
	Shaper *text.Shaper
	Palette
	TextSize unit.Sp
	Icon     struct {
		CheckBoxChecked   *widget.Icon
		CheckBoxUnchecked *widget.Icon
		RadioChecked      *widget.Icon
		RadioUnchecked    *widget.Icon
	}
	// Face selects the default typeface for text.
	Face font.Typeface

	// FingerSize is the minimum touch target size.
	FingerSize unit.Dp
}

// NewTheme constructs a theme (and underlying text shaper).
func NewTheme() *Theme {
	t := &Theme{Shaper: &text.Shaper{}}
	t.Palette = Palette{
		Fg:         rgb(0x000000),
		Bg:         rgb(0xffffff),
		ContrastBg: rgb(0x3f51b5),
		ContrastFg: rgb(0xffffff),
	}
	t.TextSize = 16

	t.Icon.CheckBoxChecked = mustIcon(widget.NewIcon(icons.ToggleCheckBox))
	t.Icon.CheckBoxUnchecked = mustIcon(widget.NewIcon(icons.ToggleCheckBoxOutlineBlank))
	t.Icon.RadioChecked = mustIcon(widget.NewIcon(icons.ToggleRadioButtonChecked))
	t.Icon.RadioUnchecked = mustIcon(widget.NewIcon(icons.ToggleRadioButtonUnchecked))

	// 38dp is on the lower end of possible finger size.
	t.FingerSize = 38

	return t
}

func (t Theme) WithPalette(p Palette) Theme {
	t.Palette = p
	return t
}

func mustIcon(ic *widget.Icon, err error) *widget.Icon {
	if err != nil {
		panic(err)
	}
	return ic
}

func rgb(c uint32) color.NRGBA {
	return argb(0xff000000 | c)
}

func argb(c uint32) color.NRGBA {
	return color.NRGBA{A: uint8(c >> 24), R: uint8(c >> 16), G: uint8(c >> 8), B: uint8(c)}
}
