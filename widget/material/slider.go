// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/internal/f32color"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// Slider is for selecting a value in a range.
func Slider(th *Theme, float *widget.Float, min, max float32) SliderStyle {
	return SliderStyle{
		Min:        min,
		Max:        max,
		Color:      th.Palette.ContrastBg,
		Float:      float,
		FingerSize: th.FingerSize,
	}
}

type SliderStyle struct {
	Min, Max float32
	Color    color.NRGBA
	Float    *widget.Float

	FingerSize unit.Value
}

func (s SliderStyle) Layout(gtx layout.Context) layout.Dimensions {
	thumbRadiusInt := gtx.Px(unit.Dp(6))
	trackWidth := float32(gtx.Px(unit.Dp(2)))
	thumbRadius := float32(thumbRadiusInt)

	size := gtx.Constraints.Min
	// Keep a minimum length so that the track is always visible.
	minLength := thumbRadiusInt + 3*thumbRadiusInt + thumbRadiusInt
	if size.X < minLength {
		size.X = minLength
	}
	size.Y = 2 * thumbRadiusInt

	// Try to expand to finger size, but only if the constraints
	// allow for it.
	touchSizePx := gtx.Px(s.FingerSize)
	if touchSizePx > gtx.Constraints.Max.Y {
		touchSizePx = gtx.Constraints.Max.Y
	}
	if size.Y < touchSizePx {
		size.Y = 2 * (touchSizePx / 2)
	}

	st := op.Save(gtx.Ops)
	op.Offset(f32.Pt(thumbRadius, 0)).Add(gtx.Ops)
	gtx.Constraints.Min = image.Pt(size.X-2*thumbRadiusInt, size.Y)
	s.Float.Layout(gtx, thumbRadiusInt, s.Min, s.Max)
	gtx.Constraints.Min.Y = size.Y
	thumbPos := thumbRadius + s.Float.Pos()
	st.Load()

	color := s.Color
	if gtx.Queue == nil {
		color = f32color.Disabled(color)
	}

	// Draw track before thumb.
	st = op.Save(gtx.Ops)
	track := f32.Rectangle{
		Min: f32.Point{
			X: thumbRadius,
			Y: float32(size.Y/2) - trackWidth/2,
		},
		Max: f32.Point{
			X: thumbPos,
			Y: float32(size.Y/2) + trackWidth/2,
		},
	}
	clip.RRect{Rect: track}.Add(gtx.Ops)
	paint.ColorOp{Color: color}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	st.Load()

	// Draw track after thumb.
	st = op.Save(gtx.Ops)
	track.Min.X = thumbPos
	track.Max.X = float32(size.X) - thumbRadius
	clip.RRect{Rect: track}.Add(gtx.Ops)
	paint.ColorOp{Color: f32color.MulAlpha(color, 96)}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	st.Load()

	// Draw thumb.
	st = op.Save(gtx.Ops)
	thumb := f32.Rectangle{
		Min: f32.Point{
			X: thumbPos - thumbRadius,
			Y: float32(size.Y/2) - thumbRadius,
		},
		Max: f32.Point{
			X: thumbPos + thumbRadius,
			Y: float32(size.Y/2) + thumbRadius,
		},
	}
	rr := thumbRadius
	clip.RRect{
		Rect: thumb,
		NE:   rr, NW: rr, SE: rr, SW: rr,
	}.Add(gtx.Ops)
	paint.ColorOp{Color: color}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	st.Load()

	return layout.Dimensions{Size: size}
}
