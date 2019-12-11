// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget"
)

type Editor struct {
	Font text.Font
	// Color is the text color.
	Color color.RGBA
	// Hint contains the text displayed when the editor is empty.
	Hint string
	// HintColor is the color of hint text.
	HintColor color.RGBA

	shaper *text.Shaper
}

func (t *Theme) Editor(hint string) Editor {
	return Editor{
		Font: text.Font{
			Size: t.TextSize,
		},
		Color:     t.Color.Text,
		shaper:    t.Shaper,
		Hint:      hint,
		HintColor: t.Color.Hint,
	}
}

func (e Editor) Layout(gtx *layout.Context, editor *widget.Editor) {
	var stack op.StackOp
	stack.Push(gtx.Ops)
	var macro op.MacroOp
	macro.Record(gtx.Ops)
	paint.ColorOp{Color: e.HintColor}.Add(gtx.Ops)
	tl := widget.Label{Alignment: editor.Alignment}
	tl.Layout(gtx, e.shaper, e.Font, e.Hint)
	macro.Stop()
	if w := gtx.Dimensions.Size.X; gtx.Constraints.Width.Min < w {
		gtx.Constraints.Width.Min = w
	}
	if h := gtx.Dimensions.Size.Y; gtx.Constraints.Height.Min < h {
		gtx.Constraints.Height.Min = h
	}
	editor.Layout(gtx, e.shaper, e.Font)
	if editor.Len() > 0 {
		paint.ColorOp{Color: e.Color}.Add(gtx.Ops)
		editor.PaintText(gtx)
	} else {
		macro.Add()
	}
	paint.ColorOp{Color: e.Color}.Add(gtx.Ops)
	editor.PaintCaret(gtx)
	stack.Pop()
}
