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
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
)

type ButtonStyle struct {
	Text string
	// Color is the text color.
	Color        color.RGBA
	Font         text.Font
	TextSize     unit.Value
	Background   color.RGBA
	CornerRadius unit.Value
	Inset        layout.Inset
	Button       *widget.Clickable
	shaper       text.Shaper
}

type ButtonLayoutStyle struct {
	Background   color.RGBA
	CornerRadius unit.Value
	Inset        layout.Inset
	Button       *widget.Clickable
}

type IconButtonStyle struct {
	Background color.RGBA
	// Color is the icon color.
	Color color.RGBA
	Icon  *widget.Icon
	// Size is the icon size.
	Size   unit.Value
	Inset  layout.Inset
	Button *widget.Clickable
}

func Button(th *Theme, button *widget.Clickable, txt string) ButtonStyle {
	return ButtonStyle{
		Text:         txt,
		Color:        rgb(0xffffff),
		CornerRadius: unit.Dp(4),
		Background:   th.Color.Primary,
		TextSize:     th.TextSize.Scale(14.0 / 16.0),
		Inset: layout.Inset{
			Top: unit.Dp(10), Bottom: unit.Dp(10),
			Left: unit.Dp(12), Right: unit.Dp(12),
		},
		Button: button,
		shaper: th.Shaper,
	}
}

func ButtonLayout(th *Theme, button *widget.Clickable) ButtonLayoutStyle {
	return ButtonLayoutStyle{
		Button:       button,
		Background:   th.Color.Primary,
		CornerRadius: unit.Dp(4),
		Inset:        layout.UniformInset(unit.Dp(12)),
	}
}

func IconButton(th *Theme, button *widget.Clickable, icon *widget.Icon) IconButtonStyle {
	return IconButtonStyle{
		Background: th.Color.Primary,
		Color:      th.Color.InvText,
		Icon:       icon,
		Size:       unit.Dp(24),
		Inset:      layout.UniformInset(unit.Dp(12)),
		Button:     button,
	}
}

// Clickable lays out a rectangular clickable widget without further
// decoration.
func Clickable(gtx layout.Context, button *widget.Clickable, w layout.Widget) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(button.Layout),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			clip.Rect{
				Rect: f32.Rectangle{Max: f32.Point{
					X: float32(gtx.Constraints.Min.X),
					Y: float32(gtx.Constraints.Min.Y),
				}},
			}.Op(gtx.Ops).Add(gtx.Ops)
			for _, c := range button.History() {
				drawInk(gtx, c)
			}
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(w),
	)
}

func (b ButtonStyle) Layout(gtx layout.Context) layout.Dimensions {
	return ButtonLayoutStyle{
		Background:   b.Background,
		CornerRadius: b.CornerRadius,
		Inset:        b.Inset,
		Button:       b.Button,
	}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		paint.ColorOp{Color: b.Color}.Add(gtx.Ops)
		return widget.Label{Alignment: text.Middle}.Layout(gtx, b.shaper, b.Font, b.TextSize, b.Text)
	})
}

func (b ButtonLayoutStyle) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	min := gtx.Constraints.Min
	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			rr := float32(gtx.Px(b.CornerRadius))
			clip.Rect{
				Rect: f32.Rectangle{Max: f32.Point{
					X: float32(gtx.Constraints.Min.X),
					Y: float32(gtx.Constraints.Min.Y),
				}},
				NE: rr, NW: rr, SE: rr, SW: rr,
			}.Op(gtx.Ops).Add(gtx.Ops)
			dims := fill(gtx, b.Background)
			for _, c := range b.Button.History() {
				drawInk(gtx, c)
			}
			return dims
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = min
			return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return b.Inset.Layout(gtx, w)
			})
		}),
		layout.Expanded(b.Button.Layout),
	)
}

func (b IconButtonStyle) Layout(gtx layout.Context) layout.Dimensions {
	return layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Min.X
			sizef := float32(size)
			rr := sizef * .5
			clip.Rect{
				Rect: f32.Rectangle{Max: f32.Point{X: sizef, Y: sizef}},
				NE:   rr, NW: rr, SE: rr, SW: rr,
			}.Op(gtx.Ops).Add(gtx.Ops)
			dims := fill(gtx, b.Background)
			for _, c := range b.Button.History() {
				drawInk(gtx, c)
			}
			return dims
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return b.Inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				size := gtx.Px(b.Size)
				if b.Icon != nil {
					b.Icon.Color = b.Color
					b.Icon.Layout(gtx, unit.Px(float32(size)))
				}
				return layout.Dimensions{
					Size: image.Point{X: size, Y: size},
				}
			})
		}),
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			pointer.Ellipse(image.Rectangle{Max: gtx.Constraints.Min}).Add(gtx.Ops)
			return b.Button.Layout(gtx)
		}),
	)
}

func drawInk(gtx layout.Context, c widget.Press) {
	d := gtx.Now().Sub(c.Time)
	t := float32(d.Seconds())
	const duration = 0.5
	if t > duration {
		return
	}
	t = t / duration
	var stack op.StackOp
	stack.Push(gtx.Ops)
	size := float32(gtx.Px(unit.Dp(700))) * t
	rr := size * .5
	col := byte(0xaa * (1 - t*t))
	ink := paint.ColorOp{Color: color.RGBA{A: col, R: col, G: col, B: col}}
	ink.Add(gtx.Ops)
	op.TransformOp{}.Offset(c.Position).Offset(f32.Point{
		X: -rr,
		Y: -rr,
	}).Add(gtx.Ops)
	clip.Rect{
		Rect: f32.Rectangle{Max: f32.Point{
			X: float32(size),
			Y: float32(size),
		}},
		NE: rr, NW: rr, SE: rr, SW: rr,
	}.Op(gtx.Ops).Add(gtx.Ops)
	paint.PaintOp{Rect: f32.Rectangle{Max: f32.Point{X: float32(size), Y: float32(size)}}}.Add(gtx.Ops)
	stack.Pop()
	op.InvalidateOp{}.Add(gtx.Ops)
}
