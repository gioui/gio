// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
)

type Label struct {
	// Face defines the text style.
	Font text.Font
	// Color is the text color.
	Color color.RGBA
	// Alignment specify the text alignment.
	Alignment text.Alignment
	// MaxLines limits the number of lines. Zero means no limit.
	MaxLines int
	Text     string

	shaper *text.Shaper
}

func (t *Theme) H1(txt string) Label {
	return t.Label(t.TextSize.Scale(96.0/16.0), txt)
}

func (t *Theme) H2(txt string) Label {
	return t.Label(t.TextSize.Scale(60.0/16.0), txt)
}

func (t *Theme) H3(txt string) Label {
	return t.Label(t.TextSize.Scale(48.0/16.0), txt)
}

func (t *Theme) H4(txt string) Label {
	return t.Label(t.TextSize.Scale(34.0/16.0), txt)
}

func (t *Theme) H5(txt string) Label {
	return t.Label(t.TextSize.Scale(24.0/16.0), txt)
}

func (t *Theme) H6(txt string) Label {
	return t.Label(t.TextSize.Scale(20.0/16.0), txt)
}

func (t *Theme) Body1(txt string) Label {
	return t.Label(t.TextSize, txt)
}

func (t *Theme) Body2(txt string) Label {
	return t.Label(t.TextSize.Scale(14.0/16.0), txt)
}

func (t *Theme) Caption(txt string) Label {
	return t.Label(t.TextSize.Scale(12.0/16.0), txt)
}

func (t *Theme) Label(size unit.Value, txt string) Label {
	return Label{
		Text:  txt,
		Color: t.Color.Text,
		Font: text.Font{
			Size: size,
		},
		shaper: t.Shaper,
	}
}

func (l Label) Layout(gtx *layout.Context) {
	paint.ColorOp{Color: l.Color}.Add(gtx.Ops)
	tl := widget.Label{Alignment: l.Alignment, MaxLines: l.MaxLines}
	tl.Layout(gtx, l.shaper, l.Font, l.Text)
}
