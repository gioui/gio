package material

import (
	"image"
	"image/color"
	"math/bits"

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
)

// DecorationsStyle provides the style elements for Decorations.
type DecorationsStyle struct {
	Decorations
	Actions    system.Action
	Title      LabelStyle
	Background color.NRGBA
	Foreground color.NRGBA
}

// Decorate a window.
func Decorate(th *Theme, actions system.Action) DecorationsStyle {
	titleStyle := Body1(th, "")
	titleStyle.Color = th.Palette.ContrastFg
	return DecorationsStyle{
		Actions:    actions,
		Title:      titleStyle,
		Background: th.Palette.ContrastBg,
		Foreground: th.Palette.ContrastFg,
	}
}

// Decorations provides window decorations.
type Decorations struct {
	actions struct {
		layout.List
		clicks []widget.Clickable
		move   gesture.Drag
		resize [8]struct {
			gesture.Hover
			gesture.Drag
		}
	}
	actioned  system.Action
	path      clip.Path
	maximized bool
}

// Layout a window with the title and actions defined in DecorationsStyle.
func (d *DecorationsStyle) Layout(gtx layout.Context, title string) layout.Dimensions {
	rec := op.Record(gtx.Ops)
	label := d.Title
	label.Text = title
	paint.ColorOp{Color: d.Foreground}.Add(gtx.Ops)
	dims := d.Decorations.layoutDecorations(gtx, d.Actions, label.Layout)
	decos := rec.Stop()
	r := clip.Rect{Max: dims.Size}
	paint.FillShape(gtx.Ops, d.Background, r.Op())
	decos.Add(gtx.Ops)
	d.Decorations.layoutResizing(gtx, d.Actions)
	return dims
}

func (d *Decorations) layoutResizing(gtx layout.Context, actions system.Action) {
	cs := gtx.Constraints.Max
	wh := gtx.Px(unit.Dp(10))
	s := []struct {
		system.Action
		image.Rectangle
	}{
		{system.ActionResizeNorth, image.Rect(0, 0, cs.X, wh)},
		{system.ActionResizeSouth, image.Rect(0, cs.Y-wh, cs.X, cs.Y)},
		{system.ActionResizeWest, image.Rect(cs.X-wh, 0, cs.X, cs.Y)},
		{system.ActionResizeEast, image.Rect(0, 0, wh, cs.Y)},
		{system.ActionResizeNorthWest, image.Rect(0, 0, wh, wh)},
		{system.ActionResizeNorthEast, image.Rect(cs.X-wh, 0, cs.X, wh)},
		{system.ActionResizeSouthWest, image.Rect(0, cs.Y-wh, wh, cs.Y)},
		{system.ActionResizeSouthEast, image.Rect(cs.X-wh, cs.Y-wh, cs.X, cs.Y)},
	}
	for i, data := range s {
		action := data.Action
		if actions&action == 0 {
			continue
		}
		rsz := &d.actions.resize[i]
		rsz.Events(gtx.Metric, gtx, gesture.Both)
		if rsz.Drag.Dragging() {
			d.actioned |= action
		}
		st := clip.Rect(data.Rectangle).Push(gtx.Ops)
		if rsz.Hover.Hovered(gtx) {
			pointer.CursorNameOp{Name: action.CursorName()}.Add(gtx.Ops)
		}
		rsz.Drag.Add(gtx.Ops)
		pass := pointer.PassOp{}.Push(gtx.Ops)
		rsz.Hover.Add(gtx.Ops)
		pass.Pop()
		st.Pop()
	}
}

func (d *Decorations) layoutDecorations(gtx layout.Context, actions system.Action, title layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.Y = 0
	inset := layout.UniformInset(unit.Dp(10))
	return layout.Flex{
		Axis:      layout.Horizontal,
		Alignment: layout.Middle,
		Spacing:   layout.SpaceBetween,
	}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			dims := inset.Layout(gtx, title)
			if actions&system.ActionMove != 0 {
				d.actions.move.Events(gtx.Metric, gtx, gesture.Both)

				st := clip.Rect{Max: dims.Size}.Push(gtx.Ops)
				d.actions.move.Add(gtx.Ops)
				if d.actions.move.Pressed() {
					d.actioned |= system.ActionMove
				}
				st.Pop()
			}
			return dims
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// Remove the unmaximize action as it is taken care of by maximize.
			actions := actions &^ system.ActionUnmaximize
			an := bits.OnesCount(uint(actions))
			if n := len(d.actions.clicks); n < an {
				d.actions.clicks = append(d.actions.clicks, make([]widget.Clickable, an-n)...)
			}
			action := system.Action(1)
			return d.actions.Layout(gtx, an, func(gtx layout.Context, idx int) layout.Dimensions {
				defer func() { action <<= 1 }()
				for actions&action == 0 {
					action <<= 1
				}
				click := &d.actions.clicks[idx]
				if click.Clicked() {
					if action == system.ActionMaximize {
						if d.maximized {
							d.maximized = false
							d.actioned |= system.ActionUnmaximize
						} else {
							d.maximized = true
							d.actioned |= system.ActionMaximize
						}
					} else {
						d.actioned |= action
					}
				}
				return Clickable(gtx, click, func(gtx layout.Context) layout.Dimensions {
					var w layout.Widget
					switch actions & action {
					case system.ActionMinimize:
						w = minimizeWindow
					case system.ActionMaximize:
						if d.maximized {
							w = maximizedWindow
						} else {
							w = maximizeWindow
						}
					case system.ActionClose:
						w = closeWindow
					default:
						return layout.Dimensions{}
					}
					return inset.Layout(gtx, w)
				})
			})
		}),
	)
}

// Perform updates the decorations as if the specified actions were
// performed by the user.
func (d *Decorations) Perform(actions system.Action) {
	if actions&system.ActionMaximize != 0 {
		d.maximized = true
	}
	if actions&(system.ActionUnmaximize|system.ActionMinimize|system.ActionFullscreen) != 0 {
		d.maximized = false
	}
}

// Actions returns the set of actions activated by the user.
func (d *Decorations) Actions() system.Action {
	a := d.actioned
	d.actioned = 0
	return a
}

var (
	winIconSize   = unit.Dp(20)
	winIconMargin = unit.Dp(4)
	winIconStroke = unit.Dp(2)
)

// minimizeWindows draws a line icon representing the minimize action.
func minimizeWindow(gtx layout.Context) layout.Dimensions {
	size := gtx.Px(winIconSize)
	size32 := float32(size)
	margin := float32(gtx.Px(winIconMargin))
	width := float32(gtx.Px(winIconStroke))
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Point{X: margin, Y: size32 - margin})
	p.LineTo(f32.Point{X: size32 - 2*margin, Y: size32 - margin})
	st := clip.Stroke{
		Path:  p.End(),
		Width: width,
	}.Op().Push(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	st.Pop()
	return layout.Dimensions{Size: image.Pt(size, size)}
}

// maximizeWindow draws a rectangle representing the maximize action.
func maximizeWindow(gtx layout.Context) layout.Dimensions {
	size := gtx.Px(winIconSize)
	size32 := float32(size)
	margin := float32(gtx.Px(winIconMargin))
	width := float32(gtx.Px(winIconStroke))
	r := clip.RRect{
		Rect: f32.Rect(margin, margin, size32-margin, size32-margin),
	}
	st := clip.Stroke{
		Path:  r.Path(gtx.Ops),
		Width: width,
	}.Op().Push(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	st.Pop()
	r.Rect.Max = f32.Pt(size32-margin, 2*margin)
	st = clip.Outline{
		Path: r.Path(gtx.Ops),
	}.Op().Push(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	st.Pop()
	return layout.Dimensions{Size: image.Pt(size, size)}
}

// maximizedWindow draws interleaved rectangles representing the un-maximize action.
func maximizedWindow(gtx layout.Context) layout.Dimensions {
	size := gtx.Px(winIconSize)
	size32 := float32(size)
	margin := float32(gtx.Px(winIconMargin))
	width := float32(gtx.Px(winIconStroke))
	r := clip.RRect{
		Rect: f32.Rect(margin, margin, size32-2*margin, size32-2*margin),
	}
	st := clip.Stroke{
		Path:  r.Path(gtx.Ops),
		Width: width,
	}.Op().Push(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	st.Pop()
	r = clip.RRect{
		Rect: f32.Rect(2*margin, 2*margin, size32-margin, size32-margin),
	}
	st = clip.Stroke{
		Path:  r.Path(gtx.Ops),
		Width: width,
	}.Op().Push(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	st.Pop()
	return layout.Dimensions{Size: image.Pt(size, size)}
}

// closeWindow draws a cross representing the close action.
func closeWindow(gtx layout.Context) layout.Dimensions {
	size := gtx.Px(winIconSize)
	size32 := float32(size)
	margin := float32(gtx.Px(winIconMargin))
	width := float32(gtx.Px(winIconStroke))
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Point{X: margin, Y: margin})
	p.LineTo(f32.Point{X: size32 - margin, Y: size32 - margin})
	p.MoveTo(f32.Point{X: size32 - margin, Y: margin})
	p.LineTo(f32.Point{X: margin, Y: size32 - margin})
	st := clip.Stroke{
		Path:  p.End(),
		Width: width,
	}.Op().Push(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	st.Pop()
	return layout.Dimensions{Size: image.Pt(size, size)}
}
