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

type Button struct {
	Text string
	// Color is the text color.
	Color        color.RGBA
	Font         text.Font
	TextSize     unit.Value
	Background   color.RGBA
	CornerRadius unit.Value
	Inset        layout.Inset
	shaper       text.Shaper
}

type ButtonLayout struct {
	Background   color.RGBA
	Color        color.RGBA
	CornerRadius unit.Value
	Inset        layout.Inset
}

type IconButton struct {
	Background color.RGBA
	Color      color.RGBA
	Icon       *Icon
	Size       unit.Value
	Padding    unit.Value
	Inset      layout.Inset
}

func (t *Theme) Button(txt string) Button {
	return Button{
		Text:         txt,
		Color:        rgb(0xffffff),
		CornerRadius: unit.Dp(4),
		Background:   t.Color.Primary,
		TextSize:     t.TextSize.Scale(14.0 / 16.0),
		Inset: layout.Inset{
			Top: unit.Dp(10), Bottom: unit.Dp(10),
			Left: unit.Dp(12), Right: unit.Dp(12),
		},
		shaper: t.Shaper,
	}
}

func (t *Theme) ButtonLayout() ButtonLayout {
	return ButtonLayout{
		Background:   t.Color.Primary,
		Color:        t.Color.InvText,
		CornerRadius: unit.Dp(4),
		Inset:        layout.UniformInset(unit.Dp(12)),
	}
}

func (t *Theme) IconButton(icon *Icon) IconButton {
	return IconButton{
		Background: t.Color.Primary,
		Color:      t.Color.InvText,
		Icon:       icon,
		Size:       unit.Dp(56),
		Padding:    unit.Dp(16),
	}
}

func (b Button) Layout(gtx *layout.Context, button *widget.Button) {
	ButtonLayout{
		Background:   b.Background,
		CornerRadius: b.CornerRadius,
		Color:        b.Color,
		Inset:        b.Inset,
	}.Layout(gtx, button, func() {
		widget.Label{Alignment: text.Middle}.Layout(gtx, b.shaper, b.Font, b.TextSize, b.Text)
	})
}

func (b ButtonLayout) Layout(gtx *layout.Context, button *widget.Button, w layout.Widget) {
	hmin := gtx.Constraints.Width.Min
	vmin := gtx.Constraints.Height.Min
	layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func() {
			rr := float32(gtx.Px(b.CornerRadius))
			clip.Rect{
				Rect: f32.Rectangle{Max: f32.Point{
					X: float32(gtx.Constraints.Width.Min),
					Y: float32(gtx.Constraints.Height.Min),
				}},
				NE: rr, NW: rr, SE: rr, SW: rr,
			}.Op(gtx.Ops).Add(gtx.Ops)
			fill(gtx, b.Background)
			for _, c := range button.History() {
				drawInk(gtx, c)
			}
		}),
		layout.Stacked(func() {
			layout.Center.Layout(gtx, func() {
				gtx.Constraints.Width.Min = hmin
				gtx.Constraints.Height.Min = vmin
				b.Inset.Layout(gtx, func() {
					paint.ColorOp{Color: b.Color}.Add(gtx.Ops)
					w()
				})
			})
			pointer.Rect(image.Rectangle{Max: gtx.Dimensions.Size}).Add(gtx.Ops)
			button.Layout(gtx)
		}),
	)
}

func (b IconButton) Layout(gtx *layout.Context, button *widget.Button) {
	layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func() {
			size := gtx.Constraints.Width.Min
			sizef := float32(size)
			rr := sizef * .5
			clip.Rect{
				Rect: f32.Rectangle{Max: f32.Point{X: sizef, Y: sizef}},
				NE:   rr, NW: rr, SE: rr, SW: rr,
			}.Op(gtx.Ops).Add(gtx.Ops)
			fill(gtx, b.Background)
			for _, c := range button.History() {
				drawInk(gtx, c)
			}
		}),
		layout.Stacked(func() {
			layout.UniformInset(b.Padding).Layout(gtx, func() {
				size := gtx.Px(b.Size) - 2*gtx.Px(b.Padding)
				if b.Icon != nil {
					b.Icon.Color = b.Color
					b.Icon.Layout(gtx, unit.Px(float32(size)))
				}
				gtx.Dimensions = layout.Dimensions{
					Size: image.Point{X: size, Y: size},
				}
			})
			pointer.Ellipse(image.Rectangle{Max: gtx.Dimensions.Size}).Add(gtx.Ops)
			button.Layout(gtx)
		}),
	)
}

func toPointF(p image.Point) f32.Point {
	return f32.Point{X: float32(p.X), Y: float32(p.Y)}
}

func drawInk(gtx *layout.Context, c widget.Click) {
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
