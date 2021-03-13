// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/internal/f32color"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

type SwitchStyle struct {
	Color struct {
		Enabled  color.NRGBA
		Disabled color.NRGBA
		Track    color.NRGBA
	}
	Switch *widget.Bool
}

// Switch is for selecting a boolean value.
func Switch(th *Theme, swtch *widget.Bool) SwitchStyle {
	sw := SwitchStyle{
		Switch: swtch,
	}
	sw.Color.Enabled = th.Palette.ContrastBg
	sw.Color.Disabled = th.Palette.Bg
	sw.Color.Track = f32color.MulAlpha(th.Palette.Fg, 0x88)
	return sw
}

// Layout updates the switch and displays it.
func (s SwitchStyle) Layout(gtx layout.Context) layout.Dimensions {
	trackWidth := gtx.Px(unit.Dp(36))
	trackHeight := gtx.Px(unit.Dp(16))
	thumbSize := gtx.Px(unit.Dp(20))
	trackOff := float32(thumbSize-trackHeight) * .5

	// Draw track.
	stack := op.Save(gtx.Ops)
	trackCorner := float32(trackHeight) / 2
	trackRect := f32.Rectangle{Max: f32.Point{
		X: float32(trackWidth),
		Y: float32(trackHeight),
	}}
	col := s.Color.Disabled
	if s.Switch.Value {
		col = s.Color.Enabled
	}
	if gtx.Queue == nil {
		col = f32color.Disabled(col)
	}
	trackColor := s.Color.Track
	op.Offset(f32.Point{Y: trackOff}).Add(gtx.Ops)
	clip.UniformRRect(trackRect, trackCorner).Add(gtx.Ops)
	paint.ColorOp{Color: trackColor}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stack.Load()

	// Draw thumb ink.
	stack = op.Save(gtx.Ops)
	inkSize := gtx.Px(unit.Dp(44))
	rr := float32(inkSize) * .5
	inkOff := f32.Point{
		X: float32(trackWidth)*.5 - rr,
		Y: -rr + float32(trackHeight)*.5 + trackOff,
	}
	op.Offset(inkOff).Add(gtx.Ops)
	gtx.Constraints.Min = image.Pt(inkSize, inkSize)
	clip.UniformRRect(f32.Rectangle{Max: layout.FPt(gtx.Constraints.Min)}, rr).Add(gtx.Ops)
	for _, p := range s.Switch.History() {
		drawInk(gtx, p)
	}
	stack.Load()

	// Compute thumb offset and color.
	stack = op.Save(gtx.Ops)
	if s.Switch.Value {
		off := trackWidth - thumbSize
		op.Offset(f32.Point{X: float32(off)}).Add(gtx.Ops)
	}

	thumbRadius := float32(thumbSize) / 2

	// Draw hover.
	if s.Switch.Hovered() {
		r := 1.7 * thumbRadius
		background := f32color.MulAlpha(s.Color.Enabled, 70)
		paint.FillShape(gtx.Ops, background,
			clip.Circle{
				Center: f32.Point{X: thumbRadius, Y: thumbRadius},
				Radius: r,
			}.Op(gtx.Ops))
	}

	// Draw thumb shadow, a translucent disc slightly larger than the
	// thumb itself.
	// Center shadow horizontally and slightly adjust its Y.
	paint.FillShape(gtx.Ops, argb(0x55000000),
		clip.Circle{
			Center: f32.Point{X: thumbRadius, Y: thumbRadius + .25},
			Radius: thumbRadius + 1,
		}.Op(gtx.Ops))

	// Draw thumb.
	paint.FillShape(gtx.Ops, col,
		clip.Circle{
			Center: f32.Point{X: thumbRadius, Y: thumbRadius},
			Radius: thumbRadius,
		}.Op(gtx.Ops))

	// Set up click area.
	stack = op.Save(gtx.Ops)
	clickSize := gtx.Px(unit.Dp(40))
	clickOff := f32.Point{
		X: (float32(trackWidth) - float32(clickSize)) * .5,
		Y: (float32(trackHeight)-float32(clickSize))*.5 + trackOff,
	}
	op.Offset(clickOff).Add(gtx.Ops)
	sz := image.Pt(clickSize, clickSize)
	pointer.Ellipse(image.Rectangle{Max: sz}).Add(gtx.Ops)
	gtx.Constraints.Min = sz
	s.Switch.Layout(gtx)
	stack.Load()

	dims := image.Point{X: trackWidth, Y: thumbSize}
	return layout.Dimensions{Size: dims}
}
