package material

import (
	"image"
	"image/color"

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
	IconColor          color.RGBA
	Size               unit.Value
	shaper             *text.Shaper
	checkedStateIcon   *Icon
	uncheckedStateIcon *Icon
}

func (c *checkable) layout(gtx *layout.Context, checked bool) {

	var icon *Icon
	if checked {
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
				icon.Color = c.IconColor
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
				paint.ColorOp{Color: c.Color}.Add(gtx.Ops)
				widget.Label{}.Layout(gtx, c.shaper, c.Font, c.Label)
			})
		})
	})

	flex.Layout(gtx, ico, lbl)
	pointer.RectAreaOp{Rect: image.Rectangle{Max: gtx.Dimensions.Size}}.Add(gtx.Ops)
}
