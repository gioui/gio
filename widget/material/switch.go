// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

type SwitchStyle struct {
	Color color.RGBA
}

func Switch(th *Theme) SwitchStyle {
	return SwitchStyle{
		Color: th.Color.Primary,
	}
}

// Layout updates the checkBox and displays it.
func (s SwitchStyle) Layout(gtx *layout.Context, swtch *widget.Bool) {
	swtch.Update(gtx)

	trackWidth := gtx.Px(unit.Dp(36))
	trackHeight := gtx.Px(unit.Dp(16))
	thumbSize := gtx.Px(unit.Dp(20))
	trackOff := float32(thumbSize-trackHeight) * .5

	// Draw track.
	var stack op.StackOp
	stack.Push(gtx.Ops)
	trackCorner := float32(trackHeight) / 2
	trackRect := f32.Rectangle{Max: f32.Point{
		X: float32(trackWidth),
		Y: float32(trackHeight),
	}}
	op.TransformOp{}.Offset(f32.Point{Y: trackOff}).Add(gtx.Ops)
	clip.Rect{
		Rect: trackRect,
		NE:   trackCorner, NW: trackCorner, SE: trackCorner, SW: trackCorner,
	}.Op(gtx.Ops).Add(gtx.Ops)
	paint.ColorOp{Color: rgb(0x9b9b9b)}.Add(gtx.Ops)
	paint.PaintOp{Rect: trackRect}.Add(gtx.Ops)
	stack.Pop()

	// Compute thumb offset and color.
	stack.Push(gtx.Ops)
	col := rgb(0xffffff)
	if swtch.Value {
		off := trackWidth - thumbSize
		op.TransformOp{}.Offset(f32.Point{X: float32(off)}).Add(gtx.Ops)
		col = s.Color
	}

	// Draw thumb shadow, a translucent disc slightly larger than the
	// thumb itself.
	var shadowStack op.StackOp
	shadowStack.Push(gtx.Ops)
	shadowSize := float32(2)
	// Center shadow horizontally and slightly adjust its Y.
	op.TransformOp{}.Offset(f32.Point{X: -shadowSize / 2, Y: -.75}).Add(gtx.Ops)
	drawDisc(gtx.Ops, float32(thumbSize)+shadowSize, argb(0x55000000))
	shadowStack.Pop()

	// Draw thumb.
	drawDisc(gtx.Ops, float32(thumbSize), col)
	stack.Pop()

	// Draw thumb ink.
	stack.Push(gtx.Ops)
	inkSize := float32(gtx.Px(unit.Dp(44)))
	rr := inkSize * .5
	inkOff := f32.Point{
		X: float32(trackWidth)*.5 - rr,
		Y: -rr + float32(trackHeight)*.5 + trackOff,
	}
	op.TransformOp{}.Offset(inkOff).Add(gtx.Ops)
	clip.Rect{
		Rect: f32.Rectangle{
			Max: f32.Point{
				X: inkSize,
				Y: inkSize,
			},
		},
		NE: rr, NW: rr, SE: rr, SW: rr,
	}.Op(gtx.Ops).Add(gtx.Ops)
	drawInk(gtx, swtch.Last)
	stack.Pop()

	// Set up click area.
	stack.Push(gtx.Ops)
	clickSize := gtx.Px(unit.Dp(40))
	clickOff := f32.Point{
		X: (float32(trackWidth) - float32(clickSize)) * .5,
		Y: (float32(trackHeight)-float32(clickSize))*.5 + trackOff,
	}
	op.TransformOp{}.Offset(clickOff).Add(gtx.Ops)
	pointer.Ellipse(image.Rectangle{
		Max: image.Point{
			X: clickSize, Y: clickSize,
		},
	}).Add(gtx.Ops)
	swtch.Layout(gtx)
	stack.Pop()

	gtx.Dimensions = layout.Dimensions{
		Size: image.Point{X: trackWidth, Y: trackHeight},
	}
}

func drawDisc(ops *op.Ops, sz float32, col color.RGBA) {
	var stack op.StackOp
	stack.Push(ops)
	rr := sz / 2
	r := f32.Rectangle{Max: f32.Point{X: sz, Y: sz}}
	clip.Rect{
		Rect: r,
		NE:   rr, NW: rr, SE: rr, SW: rr,
	}.Op(ops).Add(ops)
	paint.ColorOp{Color: col}.Add(ops)
	paint.PaintOp{Rect: r}.Add(ops)
	stack.Pop()
}
