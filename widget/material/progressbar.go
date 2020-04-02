// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

type ProgressBar struct {
	Color color.RGBA
}

func (t *Theme) ProgressBar() ProgressBar {
	return ProgressBar{
		Color: t.Color.Primary,
	}
}

func (b ProgressBar) Layout(gtx *layout.Context, progress int) {
	shader := func(width float32, color color.RGBA) {
		maxHeight := unit.Dp(4)
		rr := float32(gtx.Px(unit.Dp(2)))

		d := image.Point{X: int(width), Y: gtx.Px(maxHeight)}
		dr := f32.Rectangle{
			Max: f32.Point{X: float32(d.X), Y: float32(d.Y)},
		}

		clip.Rect{
			Rect: f32.Rectangle{Max: f32.Point{X: width, Y: float32(gtx.Px(maxHeight))}},
			NE:   rr, NW: rr, SE: rr, SW: rr,
		}.Op(gtx.Ops).Add(gtx.Ops)

		paint.ColorOp{Color: color}.Add(gtx.Ops)
		paint.PaintOp{Rect: dr}.Add(gtx.Ops)

		gtx.Dimensions = layout.Dimensions{Size: d}
	}

	if progress > 100 {
		progress = 100
	} else if progress < 0 {
		progress = 0
	}

	progressBarWidth := float32(gtx.Constraints.Width.Max)

	layout.Stack{Alignment: layout.W}.Layout(gtx,
		layout.Stacked(func() {
			// Use a transparent equivalent of progress color.
			backgroundColor := b.Color
			backgroundColor.A = 100

			shader(progressBarWidth, backgroundColor)
		}),
		layout.Stacked(func() {
			fillWidth := (progressBarWidth / 100) * float32(progress)
			shader(fillWidth, b.Color)
		}),
	)
}
