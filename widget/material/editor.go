// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"image/color"

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
	var stack op.StackOp
	stack.Push(gtx.Ops)
	var macro op.MacroOp
	macro.Record(gtx.Ops)
	paint.ColorOp{Color: e.HintColor}.Add(gtx.Ops)
	tl := widget.Label{Alignment: e.Editor.Alignment}
	dims := tl.Layout(gtx, e.shaper, e.Font, e.TextSize, e.Hint)
	macro.Stop()
	if w := dims.Size.X; gtx.Constraints.Min.X < w {
		gtx.Constraints.Min.X = w
	}
	if h := dims.Size.Y; gtx.Constraints.Min.Y < h {
		gtx.Constraints.Min.Y = h
	}
	dims = e.Editor.Layout(gtx, e.shaper, e.Font, e.TextSize)
	if e.Editor.Len() > 0 {
		paint.ColorOp{Color: e.Color}.Add(gtx.Ops)
		e.Editor.PaintText(gtx)
	} else {
		macro.Add()
	}
	paint.ColorOp{Color: e.Color}.Add(gtx.Ops)
	e.Editor.PaintCaret(gtx)
	stack.Pop()
	return dims
}
