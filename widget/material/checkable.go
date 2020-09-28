// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"image"
	"image/color"

	"gioui.org/internal/f32color"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
)

type checkable struct {
	Label              string
	Color              color.RGBA
	Font               text.Font
	TextSize           unit.Value
	IconColor          color.RGBA
	Size               unit.Value
	shaper             text.Shaper
	checkedStateIcon   *widget.Icon
	uncheckedStateIcon *widget.Icon
}

func (c *checkable) layout(gtx layout.Context, checked bool) layout.Dimensions {
	var icon *widget.Icon
	if checked {
		icon = c.checkedStateIcon
	} else {
		icon = c.uncheckedStateIcon
	}

	min := gtx.Constraints.Min
	dims := layout.Flex{Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(2)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					size := gtx.Px(c.Size)
					icon.Color = c.IconColor
					if gtx.Queue == nil {
						icon.Color = f32color.MulAlpha(icon.Color, 150)
					}
					icon.Layout(gtx, unit.Px(float32(size)))
					return layout.Dimensions{
						Size: image.Point{X: size, Y: size},
					}
				})
			})
		}),

		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = min
			return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.UniformInset(unit.Dp(2)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					paint.ColorOp{Color: c.Color}.Add(gtx.Ops)
					return widget.Label{}.Layout(gtx, c.shaper, c.Font, c.TextSize, c.Label)
				})
			})
		}),
	)
	pointer.Rect(image.Rectangle{Max: dims.Size}).Add(gtx.Ops)
	return dims
}
