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
	Background   color.RGBA
	CornerRadius unit.Value
	shaper       *text.Shaper
}

type IconButton struct {
	Background color.RGBA
	Color      color.RGBA
	Icon       *Icon
	Size       unit.Value
	Padding    unit.Value
}

func (t *Theme) Button(txt string) Button {
	return Button{
		Text:       txt,
		Color:      rgb(0xffffff),
		Background: t.Color.Primary,
		Font: text.Font{
			Size: t.TextSize.Scale(14.0 / 16.0),
		},
		shaper: t.Shaper,
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
	col := b.Color
	bgcol := b.Background
	hmin := gtx.Constraints.Width.Min
	vmin := gtx.Constraints.Height.Min
	layout.Stack{Alignment: layout.Center}.Layout(gtx,
		layout.Expanded(func() {
			rr := float32(gtx.Px(unit.Dp(4)))
			clip.Rect{
				Rect: f32.Rectangle{Max: f32.Point{
					X: float32(gtx.Constraints.Width.Min),
					Y: float32(gtx.Constraints.Height.Min),
				}},
				NE: rr, NW: rr, SE: rr, SW: rr,
			}.Op(gtx.Ops).Add(gtx.Ops)
			fill(gtx, bgcol)
			for _, c := range button.History() {
				drawInk(gtx, c)
			}
		}),
		layout.Stacked(func() {
			gtx.Constraints.Width.Min = hmin
			gtx.Constraints.Height.Min = vmin
			layout.Align(layout.Center).Layout(gtx, func() {
				layout.Inset{Top: unit.Dp(10), Bottom: unit.Dp(10), Left: unit.Dp(12), Right: unit.Dp(12)}.Layout(gtx, func() {
					paint.ColorOp{Color: col}.Add(gtx.Ops)
					widget.Label{}.Layout(gtx, b.shaper, b.Font, b.Text)
				})
			})
			pointer.Rect(image.Rectangle{Max: gtx.Dimensions.Size}).Add(gtx.Ops)
			button.Layout(gtx)
		}),
	)
}

func (b IconButton) Layout(gtx *layout.Context, button *widget.Button) {
	layout.Stack{}.Layout(gtx,
		layout.Expanded(func() {
			size := float32(gtx.Constraints.Width.Min)
			rr := float32(size) * .5
			clip.Rect{
				Rect: f32.Rectangle{Max: f32.Point{X: size, Y: size}},
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

func toRectF(r image.Rectangle) f32.Rectangle {
	return f32.Rectangle{
		Min: toPointF(r.Min),
		Max: toPointF(r.Max),
	}
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
