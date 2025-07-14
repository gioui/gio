// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"image"
	"image/color"

	"gioui.org/internal/f32color"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// Slider is for selecting a value in a range.
func Slider(th *Theme, float *widget.Float) SliderStyle {
	return SliderStyle{
		Color:      th.Palette.ContrastBg,
		Float:      float,
		FingerSize: th.FingerSize,
	}
}

type SliderStyle struct {
	Axis  layout.Axis
	Color color.NRGBA
	Float *widget.Float

	FingerSize unit.Dp
}

func (s SliderStyle) Layout(gtx layout.Context) layout.Dimensions {
	const thumbRadius unit.Dp = 6
	tr := gtx.Dp(thumbRadius)
	trackWidth := gtx.Dp(2)

	axis := s.Axis
	// Keep a minimum length so that the track is always visible.
	minLength := tr + 3*tr + tr
	// Try to expand to finger size, but only if the constraints
	// allow for it.
	touchSizePx := min(gtx.Dp(s.FingerSize), axis.Convert(gtx.Constraints.Max).Y)
	sizeMain := max(axis.Convert(gtx.Constraints.Min).X, minLength)
	sizeCross := max(2*tr, touchSizePx)
	size := axis.Convert(image.Pt(sizeMain, sizeCross))

	o := axis.Convert(image.Pt(tr, 0))
	trans := op.Offset(o).Push(gtx.Ops)
	gtx.Constraints.Min = axis.Convert(image.Pt(sizeMain-2*tr, sizeCross))
	dims := s.Float.Layout(gtx, axis, thumbRadius)
	gtx.Constraints.Min = gtx.Constraints.Min.Add(axis.Convert(image.Pt(0, sizeCross)))
	thumbPos := tr + int(s.Float.Value*float32(axis.Convert(dims.Size).X))
	trans.Pop()

	color := s.Color
	if !gtx.Enabled() {
		color = f32color.Disabled(color)
	}

	rect := func(minx, miny, maxx, maxy int) image.Rectangle {
		r := image.Rect(minx, miny, maxx, maxy)
		if axis == layout.Vertical {
			r.Max.X, r.Min.X = sizeMain-r.Min.X, sizeMain-r.Max.X
		}
		r.Min = axis.Convert(r.Min)
		r.Max = axis.Convert(r.Max)
		return r
	}

	// Draw track before thumb.
	track := rect(
		tr, sizeCross/2-trackWidth/2,
		thumbPos, sizeCross/2+trackWidth/2,
	)
	paint.FillShape(gtx.Ops, color, clip.Rect(track).Op())

	// Draw track after thumb.
	track = rect(
		thumbPos, axis.Convert(track.Min).Y,
		sizeMain-tr, axis.Convert(track.Max).Y,
	)
	paint.FillShape(gtx.Ops, f32color.MulAlpha(color, 96), clip.Rect(track).Op())

	// Draw thumb.
	pt := image.Pt(thumbPos, sizeCross/2)
	thumb := rect(
		pt.X-tr, pt.Y-tr,
		pt.X+tr, pt.Y+tr,
	)
	paint.FillShape(gtx.Ops, color, clip.Ellipse(thumb).Op(gtx.Ops))

	return layout.Dimensions{Size: size}
}
