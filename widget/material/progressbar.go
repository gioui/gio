// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"image"
	"image/color"

	"gioui.org/internal/f32color"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

type ProgressBarStyle struct {
	Color      color.NRGBA
	Height     unit.Dp
	Radius     unit.Dp
	TrackColor color.NRGBA
	Progress   float32
}

func ProgressBar(th *Theme, progress float32) ProgressBarStyle {
	return ProgressBarStyle{
		Progress:   progress,
		Height:     unit.Dp(4),
		Radius:     unit.Dp(2),
		Color:      th.Palette.ContrastBg,
		TrackColor: f32color.MulAlpha(th.Palette.Fg, 0x88),
	}
}

func (p ProgressBarStyle) Layout(gtx layout.Context) layout.Dimensions {
	shader := func(width int, color color.NRGBA) layout.Dimensions {
		d := image.Point{X: width, Y: gtx.Dp(p.Height)}
		rr := gtx.Dp(p.Radius)

		defer clip.UniformRRect(image.Rectangle{Max: image.Pt(width, d.Y)}, rr).Push(gtx.Ops).Pop()
		paint.ColorOp{Color: color}.Add(gtx.Ops)
		paint.PaintOp{}.Add(gtx.Ops)

		return layout.Dimensions{Size: d}
	}

	progressBarWidth := gtx.Constraints.Max.X
	return layout.Stack{Alignment: layout.W}.Layout(gtx,
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return shader(progressBarWidth, p.TrackColor)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			fillWidth := int(float32(progressBarWidth) * clamp1(p.Progress))
			fillColor := p.Color
			if !gtx.Enabled() {
				fillColor = f32color.Disabled(fillColor)
			}
			if fillWidth < int(p.Radius*2) {
				fillWidth = int(p.Radius * 2)
			}
			return shader(fillWidth, fillColor)
		}),
	)
}

// clamp1 limits v to range [0..1].
func clamp1(v float32) float32 {
	if v >= 1 {
		return 1
	} else if v <= 0 {
		return 0
	} else {
		return v
	}
}
