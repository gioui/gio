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
		Enabled  color.RGBA
		Disabled color.RGBA
	}
	Switch *widget.Bool
}

func Switch(th *Theme, swtch *widget.Bool) SwitchStyle {
	sw := SwitchStyle{
		Switch: swtch,
	}
	sw.Color.Enabled = th.Color.Primary
	sw.Color.Disabled = rgb(0xffffff)
	return sw
}

// Layout updates the checkBox and displays it.
func (s SwitchStyle) Layout(gtx layout.Context) layout.Dimensions {
	trackWidth := gtx.Px(unit.Dp(36))
	trackHeight := gtx.Px(unit.Dp(16))
	thumbSize := gtx.Px(unit.Dp(20))
	trackOff := float32(thumbSize-trackHeight) * .5

	// Draw track.
	stack := op.Push(gtx.Ops)
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
		col = f32color.MulAlpha(col, 150)
	}
	trackColor := f32color.MulAlpha(col, 150)
	op.Offset(f32.Point{Y: trackOff}).Add(gtx.Ops)
	clip.RRect{
		Rect: trackRect,
		NE:   trackCorner, NW: trackCorner, SE: trackCorner, SW: trackCorner,
	}.Add(gtx.Ops)
	paint.ColorOp{Color: trackColor}.Add(gtx.Ops)
	paint.PaintOp{Rect: trackRect}.Add(gtx.Ops)
	stack.Pop()

	// Draw thumb ink.
	stack = op.Push(gtx.Ops)
	inkSize := gtx.Px(unit.Dp(44))
	rr := float32(inkSize) * .5
	inkOff := f32.Point{
		X: float32(trackWidth)*.5 - rr,
		Y: -rr + float32(trackHeight)*.5 + trackOff,
	}
	op.Offset(inkOff).Add(gtx.Ops)
	gtx.Constraints.Min = image.Pt(inkSize, inkSize)
	clip.RRect{
		Rect: f32.Rectangle{
			Max: layout.FPt(gtx.Constraints.Min),
		},
		NE: rr, NW: rr, SE: rr, SW: rr,
	}.Add(gtx.Ops)
	for _, p := range s.Switch.History() {
		drawInk(gtx, p)
	}
	stack.Pop()

	// Compute thumb offset and color.
	stack = op.Push(gtx.Ops)
	if s.Switch.Value {
		off := trackWidth - thumbSize
		op.Offset(f32.Point{X: float32(off)}).Add(gtx.Ops)
	}

	// Draw thumb shadow, a translucent disc slightly larger than the
	// thumb itself.
	shadowStack := op.Push(gtx.Ops)
	shadowSize := float32(2)
	// Center shadow horizontally and slightly adjust its Y.
	op.Offset(f32.Point{X: -shadowSize / 2, Y: -.75}).Add(gtx.Ops)
	drawDisc(gtx.Ops, float32(thumbSize)+shadowSize, argb(0x55000000))
	shadowStack.Pop()

	// Draw thumb.
	drawDisc(gtx.Ops, float32(thumbSize), col)
	stack.Pop()

	// Set up click area.
	stack = op.Push(gtx.Ops)
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
	stack.Pop()

	dims := image.Point{X: trackWidth, Y: thumbSize}
	return layout.Dimensions{Size: dims}
}

func drawDisc(ops *op.Ops, sz float32, col color.RGBA) {
	defer op.Push(ops).Pop()
	rr := sz / 2
	r := f32.Rectangle{Max: f32.Point{X: sz, Y: sz}}
	clip.RRect{
		Rect: r,
		NE:   rr, NW: rr, SE: rr, SW: rr,
	}.Add(ops)
	paint.ColorOp{Color: col}.Add(ops)
	paint.PaintOp{Rect: r}.Add(ops)
}
