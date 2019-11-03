// SPDX-License-Identifier: Unlicense OR MIT

// Package material implements the Material design.
package material

import (
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"image"
	"image/color"
)

type CheckBox struct {
	Text string
	// Color is the text color.
	Color              color.RGBA
	Font               text.Font
	IconColor          color.RGBA
	Size               unit.Value
	shaper             *text.Shaper
	checkedStateIcon   *Icon
	uncheckedStateIcon *Icon
}

func (t *Theme) CheckBox(txt string) CheckBox {
	return CheckBox{
		Text:      txt,
		Color:     t.Color.Text,
		IconColor: t.Color.Primary,
		Font: text.Font{
			Size: t.TextSize.Scale(14.0 / 16.0),
		},
		Size:               unit.Dp(26),
		shaper:             t.Shaper,
		checkedStateIcon:   t.checkedStateIcon,
		uncheckedStateIcon: t.uncheckedStateIcon,
	}
}

func (c CheckBox) Layout(gtx *layout.Context, checkBox *widget.CheckBox) {

	textColor := c.Color
	iconColor := c.IconColor

	var icon *Icon
	if checkBox.Checked(gtx) {
		icon = c.checkedStateIcon
	} else {
		icon = c.uncheckedStateIcon
	}

	hmin := gtx.Constraints.Width.Min
	vmin := gtx.Constraints.Height.Min

	flex := layout.Flex{Alignment: layout.Middle}

	ico := flex.Rigid(gtx, func() {
		layout.Align(layout.Center).Layout(gtx, func() {
			layout.UniformInset(unit.Dp(2)).Layout(gtx, func() {
				size := gtx.Px(c.Size)
				icon.Color = iconColor
				icon.Layout(gtx, unit.Px(float32(size)))
				gtx.Dimensions = layout.Dimensions{
					Size: image.Point{X: size, Y: size},
				}
			})
		})
	})

	lbl := flex.Rigid(gtx, func() {
		gtx.Constraints.Width.Min = hmin
		gtx.Constraints.Height.Min = vmin
		layout.Align(layout.Start).Layout(gtx, func() {
			layout.UniformInset(unit.Dp(2)).Layout(gtx, func() {
				paint.ColorOp{Color: textColor}.Add(gtx.Ops)
				widget.Label{}.Layout(gtx, c.shaper, c.Font, c.Text)
			})
		})
	})

	flex.Layout(gtx, ico, lbl)
	pointer.RectAreaOp{Rect: image.Rectangle{Max: gtx.Dimensions.Size}}.Add(gtx.Ops)
	checkBox.Layout(gtx)
}
