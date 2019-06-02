// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"
	"math"

	"gioui.org/ui"
)

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

type Insets struct {
	Top, Right, Bottom, Left float32

	cs Constraints
}

func (in *Insets) Begin(ops *ui.Ops, cs Constraints) Constraints {
	in.cs = cs
	mcs := cs
	t, r, b, l := int(math.Round(float64(in.Top))), int(math.Round(float64(in.Right))), int(math.Round(float64(in.Bottom))), int(math.Round(float64(in.Left)))
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
	ui.OpPush{}.Add(ops)
	ui.OpTransform{Transform: ui.Offset(toPointF(image.Point{X: l, Y: t}))}.Add(ops)
	return mcs
}

func (in *Insets) End(ops *ui.Ops, dims Dimens) Dimens {
	ui.OpPop{}.Add(ops)
	t, r, b, l := int(math.Round(float64(in.Top))), int(math.Round(float64(in.Right))), int(math.Round(float64(in.Bottom))), int(math.Round(float64(in.Left)))
	return Dimens{
		Size:     in.cs.Constrain(dims.Size.Add(image.Point{X: r + l, Y: t + b})),
		Baseline: dims.Baseline + t,
	}
}

func EqualInsets(v float32) Insets {
	return Insets{Top: v, Right: v, Bottom: v, Left: v}
}

func isInf(v ui.Value) bool {
	return math.IsInf(float64(v.V), 1)
}

type Sized struct {
	Width, Height float32
}

func (s Sized) Constrain(cs Constraints) Constraints {
	if h := int(s.Height + 0.5); h != 0 {
		if cs.Height.Min < h {
			cs.Height.Min = h
		}
		if h < cs.Height.Max {
			cs.Height.Max = h
		}
	}
	if w := int(s.Width + .5); w != 0 {
		if cs.Width.Min < w {
			cs.Width.Min = w
		}
		if w < cs.Width.Max {
			cs.Width.Max = w
		}
	}
	return cs
}

type Align struct {
	Alignment Direction
	cs        Constraints
}

func (a *Align) Begin(ops *ui.Ops, cs Constraints) Constraints {
	a.cs = cs
	ops.Begin()
	return cs.Loose()
}

func (a *Align) End(ops *ui.Ops, dims Dimens) Dimens {
	block := ops.End()
	sz := dims.Size
	if a.cs.Width.Max != ui.Inf {
		sz.X = a.cs.Width.Max
	}
	if a.cs.Height.Max != ui.Inf {
		sz.Y = a.cs.Height.Max
	}
	var p image.Point
	switch a.Alignment {
	case N, S, Center:
		p.X = (sz.X - dims.Size.X) / 2
	case NE, SE, E:
		p.X = sz.X - dims.Size.X
	}
	switch a.Alignment {
	case W, Center, E:
		p.Y = (sz.Y - dims.Size.Y) / 2
	case SW, S, SE:
		p.Y = sz.Y - dims.Size.Y
	}
	ui.OpPush{}.Add(ops)
	ui.OpTransform{Transform: ui.Offset(toPointF(p))}.Add(ops)
	block.Add(ops)
	ui.OpPop{}.Add(ops)
	return Dimens{
		Size:     sz,
		Baseline: dims.Baseline,
	}
}
