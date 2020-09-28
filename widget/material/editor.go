// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"image/color"

	"gioui.org/internal/f32color"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
)

type EditorStyle struct {
	Font     text.Font
	TextSize unit.Value
	// Color is the text color.
	Color color.RGBA
	// Hint contains the text displayed when the editor is empty.
	Hint string
	// HintColor is the color of hint text.
	HintColor color.RGBA
	Editor    *widget.Editor

	shaper text.Shaper
}

func Editor(th *Theme, editor *widget.Editor, hint string) EditorStyle {
	return EditorStyle{
		Editor:    editor,
		TextSize:  th.TextSize,
		Color:     th.Color.Text,
		shaper:    th.Shaper,
		Hint:      hint,
		HintColor: th.Color.Hint,
	}
}

func (e EditorStyle) Layout(gtx layout.Context) layout.Dimensions {
	defer op.Push(gtx.Ops).Pop()
	macro := op.Record(gtx.Ops)
	paint.ColorOp{Color: e.HintColor}.Add(gtx.Ops)
	tl := widget.Label{Alignment: e.Editor.Alignment}
	dims := tl.Layout(gtx, e.shaper, e.Font, e.TextSize, e.Hint)
	call := macro.Stop()
	if w := dims.Size.X; gtx.Constraints.Min.X < w {
		gtx.Constraints.Min.X = w
	}
	if h := dims.Size.Y; gtx.Constraints.Min.Y < h {
		gtx.Constraints.Min.Y = h
	}
	dims = e.Editor.Layout(gtx, e.shaper, e.Font, e.TextSize)
	disabled := gtx.Queue == nil
	if e.Editor.Len() > 0 {
		textColor := e.Color
		if disabled {
			textColor = f32color.MulAlpha(textColor, 150)
		}
		paint.ColorOp{Color: textColor}.Add(gtx.Ops)
		e.Editor.PaintText(gtx)
	} else {
		call.Add(gtx.Ops)
	}
	if !disabled {
		paint.ColorOp{Color: e.Color}.Add(gtx.Ops)
		e.Editor.PaintCaret(gtx)
	}
	return dims
}
