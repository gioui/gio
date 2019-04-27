// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"
	"math"

	"gioui.org/ui"
)

type Widget interface {
	Layout(ops *ui.Ops, cs Constraints) Dimens
}

type Constraints struct {
	Width  Constraint
	Height Constraint
}

type Constraint struct {
	Min, Max int
}

type Dimens struct {
	Size     image.Point
	Baseline int
}

type Axis uint8

type F func(ops *ui.Ops, cs Constraints) Dimens

const (
	Horizontal Axis = iota
	Vertical
)

func (c Constraint) Constrain(v int) int {
	if v < c.Min {
		return c.Min
	} else if v > c.Max {
		return c.Max
	}
	return v
}

func (c Constraints) Constrain(p image.Point) image.Point {
	return image.Point{X: c.Width.Constrain(p.X), Y: c.Height.Constrain(p.Y)}
}

func (c Constraints) Expand() Constraints {
	return Constraints{Width: c.Width.Expand(), Height: c.Height.Expand()}
}

func (c Constraint) Expand() Constraint {
	return Constraint{Min: c.Max, Max: c.Max}
}
func (c Constraints) Loose() Constraints {
	return Constraints{Width: c.Width.Loose(), Height: c.Height.Loose()}
}

func (c Constraint) Loose() Constraint {
	return Constraint{Max: c.Max}
}

// ExactConstraints returns the constraints that exactly represents the
// given dimensions.
func ExactConstraints(size image.Point) Constraints {
	return Constraints{
		Width:  Constraint{Min: size.X, Max: size.X},
		Height: Constraint{Min: size.Y, Max: size.Y},
	}
}

func (f F) Layout(ops *ui.Ops, cs Constraints) Dimens {
	return f(ops, cs)
}

type Margins struct {
	Top, Right, Bottom, Left ui.Value
}

func Margin(c *ui.Config, m Margins, w Widget) Widget {
	return F(func(ops *ui.Ops, cs Constraints) Dimens {
		mcs := cs
		t, r, b, l := int(c.Pixels(m.Top)+0.5), int(c.Pixels(m.Right)+0.5), int(c.Pixels(m.Bottom)+0.5), int(c.Pixels(m.Left)+0.5)
		if mcs.Width.Max != ui.Inf {
			mcs.Width.Min -= l + r
			mcs.Width.Max -= l + r
			if mcs.Width.Min < 0 {
				mcs.Width.Min = 0
			}
			if mcs.Width.Max < mcs.Width.Min {
				mcs.Width.Max = mcs.Width.Min
			}
		}
		if mcs.Height.Max != ui.Inf {
			mcs.Height.Min -= t + b
			mcs.Height.Max -= t + b
			if mcs.Height.Min < 0 {
				mcs.Height.Min = 0
			}
			if mcs.Height.Max < mcs.Height.Min {
				mcs.Height.Max = mcs.Height.Min
			}
		}
		ops.Begin()
		ui.OpTransform{Transform: ui.Offset(toPointF(image.Point{X: l, Y: t}))}.Add(ops)
		dims := w.Layout(ops, mcs)
		ops.End().Add(ops)
		return Dimens{
			Size:     cs.Constrain(dims.Size.Add(image.Point{X: r + l, Y: t + b})),
			Baseline: dims.Baseline + t,
		}
	})
}

func EqualMargins(v ui.Value) Margins {
	return Margins{Top: v, Right: v, Bottom: v, Left: v}
}

func isInf(v ui.Value) bool {
	return math.IsInf(float64(v.V), 1)
}

func Capped(c *ui.Config, maxWidth, maxHeight ui.Value, wt Widget) Widget {
	return F(func(ops *ui.Ops, cs Constraints) Dimens {
		if !isInf(maxWidth) {
			mw := int(c.Pixels(maxWidth) + .5)
			if mw < cs.Width.Min {
				mw = cs.Width.Min
			}
			if mw < cs.Width.Max {
				cs.Width.Max = mw
			}
		}
		if !isInf(maxHeight) {
			mh := int(c.Pixels(maxHeight) + 0.5)
			if mh < cs.Height.Min {
				mh = cs.Height.Min
			}
			if mh < cs.Height.Max {
				cs.Height.Max = mh
			}
		}
		return wt.Layout(ops, cs)
	})
}

func Sized(c *ui.Config, width, height ui.Value, wt Widget) Widget {
	return F(func(ops *ui.Ops, cs Constraints) Dimens {
		if h := int(c.Pixels(height) + 0.5); h != 0 {
			if cs.Height.Min < h {
				cs.Height.Min = h
			}
			if h < cs.Height.Max {
				cs.Height.Max = h
			}
		}
		if w := int(c.Pixels(width) + .5); w != 0 {
			if cs.Width.Min < w {
				cs.Width.Min = w
			}
			if w < cs.Width.Max {
				cs.Width.Max = w
			}
		}
		return wt.Layout(ops, cs)
	})
}

func Expand(w Widget) Widget {
	return F(func(ops *ui.Ops, cs Constraints) Dimens {
		if cs.Height.Max != ui.Inf {
			cs.Height.Min = cs.Height.Max
		}
		if cs.Width.Max != ui.Inf {
			cs.Width.Min = cs.Width.Max
		}
		return w.Layout(ops, cs)
	})
}

func Align(alignment Direction, w Widget) Widget {
	return F(func(ops *ui.Ops, cs Constraints) Dimens {
		ops.Begin()
		dims := w.Layout(ops, cs.Loose())
		block := ops.End()
		sz := dims.Size
		if cs.Width.Max != ui.Inf {
			sz.X = cs.Width.Max
		}
		if cs.Height.Max != ui.Inf {
			sz.Y = cs.Height.Max
		}
		var p image.Point
		switch alignment {
		case N, S, Center:
			p.X = (sz.X - dims.Size.X) / 2
		case NE, SE, E:
			p.X = sz.X - dims.Size.X
		}
		switch alignment {
		case W, Center, E:
			p.Y = (sz.Y - dims.Size.Y) / 2
		case SW, S, SE:
			p.Y = sz.Y - dims.Size.Y
		}
		ops.Begin()
		ui.OpTransform{Transform: ui.Offset(toPointF(p))}.Add(ops)
		block.Add(ops)
		ops.End().Add(ops)
		return Dimens{
			Size:     sz,
			Baseline: dims.Baseline,
		}
	})
}
