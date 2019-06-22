// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/ui"
	"gioui.org/ui/draw"
	"gioui.org/ui/gesture"
	"gioui.org/ui/input"
	"gioui.org/ui/pointer"
)

type scrollChild struct {
	size  image.Point
	block ui.BlockOp
}

type List struct {
	Config             *ui.Config
	Inputs             input.Events
	Axis               Axis
	Invert             bool
	CrossAxisAlignment CrossAxisAlignment

	// The distance scrolled since last call to Init.
	Distance int

	ops       *ui.Ops
	scroll    gesture.Scroll
	scrollDir int

	offset int
	first  int

	cs  Constraints
	len int

	maxSize  int
	children []scrollChild
	dir      iterationDir
}

type iterationDir uint8

const (
	iterateNone iterationDir = iota
	iterateForward
	iterateBackward
)

func (l *List) Init(ops *ui.Ops, cs Constraints, len int) {
	l.update()
	l.ops = ops
	l.dir = iterateNone
	l.maxSize = 0
	l.children = l.children[:0]
	l.cs = cs
	l.len = len
	if l.first > len {
		l.first = len
	}
	ops.Begin()
}

func (l *List) Dragging() bool {
	return l.scroll.Dragging()
}

func (l *List) update() {
	l.Distance = 0
	d := l.scroll.Scroll(l.Config, l.Inputs, gesture.Axis(l.Axis))
	if l.Invert {
		d = -d
	}
	l.scrollDir = d
	l.Distance += d
	l.offset += d
}

func (l *List) Next() (int, Constraints, bool) {
	if l.dir != iterateNone {
		panic("a previous Next was not finished with Elem")
	}
	i, ok := l.next()
	if l.Invert {
		i = l.len - 1 - i
	}
	var cs Constraints
	if ok {
		cs = axisConstraints(l.Axis, Constraint{Max: ui.Inf}, l.crossConstraintChild(l.cs))
		l.ops.Begin()
		ui.LayerOp{}.Add(l.ops)
	}
	return i, cs, ok
}

func (l *List) next() (int, bool) {
	mainc := axisMainConstraint(l.Axis, l.cs)
	if l.offset <= 0 {
		if l.first > 0 {
			l.dir = iterateBackward
			return l.first - 1, true
		}
		l.offset = 0
	}
	if l.maxSize-l.offset < mainc.Max {
		i := l.first + len(l.children)
		if i < l.len {
			l.dir = iterateForward
			return i, true
		}
		missing := mainc.Max - (l.maxSize - l.offset)
		if missing > l.offset {
			missing = l.offset
		}
		l.offset -= missing
	}
	return 0, false
}

func (l *List) End(dims Dimens) {
	block := l.ops.End()
	child := scrollChild{dims.Size, block}
	switch l.dir {
	case iterateForward:
		mainSize := axisMain(l.Axis, child.size)
		l.maxSize += mainSize
		l.children = append(l.children, child)
	case iterateBackward:
		l.first--
		mainSize := axisMain(l.Axis, child.size)
		l.offset += mainSize
		l.maxSize += mainSize
		l.children = append([]scrollChild{child}, l.children...)
	default:
		panic("call Next before End")
	}
	l.dir = iterateNone
}

func (l *List) Layout() Dimens {
	mainc := axisMainConstraint(l.Axis, l.cs)
	for len(l.children) > 0 {
		sz := l.children[0].size
		mainSize := axisMain(l.Axis, sz)
		if l.offset <= mainSize {
			break
		}
		l.first++
		l.offset -= mainSize
		l.children = l.children[1:]
	}
	size := -l.offset
	var maxCross int
	for i, child := range l.children {
		sz := child.size
		if c := axisCross(l.Axis, sz); c > maxCross {
			maxCross = c
		}
		size += axisMain(l.Axis, sz)
		if size >= mainc.Max {
			l.children = l.children[:i+1]
			break
		}
	}
	ops := l.ops
	pos := -l.offset
	for _, child := range l.children {
		sz := child.size
		var cross int
		switch l.CrossAxisAlignment {
		case End:
			cross = maxCross - axisCross(l.Axis, sz)
		case Center:
			cross = (maxCross - axisCross(l.Axis, sz)) / 2
		}
		childSize := axisMain(l.Axis, sz)
		max := childSize + pos
		if max > mainc.Max {
			max = mainc.Max
		}
		min := pos
		if min < 0 {
			min = 0
		}
		transPos := pos
		if l.Invert {
			transPos = mainc.Max - transPos - childSize
			min, max = mainc.Max-max, mainc.Max-min
		}
		r := image.Rectangle{
			Min: axisPoint(l.Axis, min, -ui.Inf),
			Max: axisPoint(l.Axis, max, ui.Inf),
		}
		ui.PushOp{}.Add(ops)
		draw.RectClip(r).Add(ops)
		ui.TransformOp{
			Transform: ui.Offset(toPointF(axisPoint(l.Axis, transPos, cross))),
		}.Add(ops)
		child.block.Add(ops)
		ui.PopOp{}.Add(ops)
		pos += childSize
	}
	atStart := l.first == 0 && l.offset <= 0
	atEnd := l.first+len(l.children) == l.len && mainc.Max >= pos
	if atStart && l.scrollDir < 0 || atEnd && l.scrollDir > 0 {
		l.scroll.Stop()
	}
	dims := axisPoint(l.Axis, mainc.Constrain(pos), maxCross)
	block := ops.End()
	pointer.AreaRect(dims).Add(ops)
	l.scroll.Add(ops)
	block.Add(ops)
	return Dimens{Size: dims}
}

func (l *List) crossConstraintChild(cs Constraints) Constraint {
	c := axisCrossConstraint(l.Axis, cs)
	switch l.CrossAxisAlignment {
	case Stretch:
		c.Min = c.Max
	default:
		c.Min = 0
	}
	return c
}
