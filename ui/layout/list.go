// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/ui"
	"gioui.org/ui/draw"
	"gioui.org/ui/gesture"
	"gioui.org/ui/pointer"
)

type scrollChild struct {
	size  image.Point
	block ui.OpBlock
}

type List struct {
	Axis Axis

	CrossAxisAlignment CrossAxisAlignment

	// The distance scrolled since last call to Init.
	Distance int

	area      gesture.Rect
	scroll    gesture.Scroll
	scrollDir int

	offset int
	first  int

	ops *ui.Ops
	cs  Constraints
	len int

	maxSize  int
	children []scrollChild
	elem     func(w Widget)

	size image.Point
}

func (l *List) Init(ops *ui.Ops, cs Constraints, len int) (int, bool) {
	l.maxSize = 0
	l.children = l.children[:0]
	l.ops = ops
	l.cs = cs
	l.len = len
	l.elem = nil
	if l.first > len {
		l.first = len
	}
	if len == 0 {
		return 0, false
	}
	l.scroll.Op(ops, &l.area)
	return l.Index()
}

func (l *List) Dragging() bool {
	return l.scroll.Dragging()
}

func (l *List) Scroll(c *ui.Config, q pointer.Events) {
	l.Distance = 0
	d := l.scroll.Scroll(c, q, gesture.Axis(l.Axis))
	l.scrollDir = d
	l.Distance += d
	l.offset += d
}

func (l *List) Index() (int, bool) {
	i, ok := l.next()
	if !ok {
		l.draw()
	}
	return i, ok
}

func (l *List) Layout() Dimens {
	l.area.Size = l.size
	return Dimens{Size: l.size}
}

func (l *List) next() (int, bool) {
	mainc := axisMainConstraint(l.Axis, l.cs)
	if l.offset <= 0 {
		if l.first > 0 {
			l.elem = l.backward
			return l.first - 1, true
		}
		l.offset = 0
	}
	if l.maxSize-l.offset < mainc.Max {
		i := l.first + len(l.children)
		if i < l.len {
			l.elem = l.forward
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

func (l *List) Elem(w Widget) {
	l.elem(w)
}

func (l *List) backward(w Widget) {
	l.first--
	child := l.add(w)
	mainSize := axisMain(l.Axis, child.size)
	l.offset += mainSize
	l.maxSize += mainSize
	l.children = append([]scrollChild{child}, l.children...)
}

func (l *List) forward(w Widget) {
	child := l.add(w)
	mainSize := axisMain(l.Axis, child.size)
	l.maxSize += mainSize
	l.children = append(l.children, child)
}

func (l *List) add(w Widget) scrollChild {
	subcs := axisConstraints(l.Axis, Constraint{Max: ui.Inf}, l.crossConstraintChild(l.cs))
	l.ops.Begin()
	ui.OpLayer{}.Add(l.ops)
	dims := w.Layout(l.ops, subcs)
	block := l.ops.End()
	return scrollChild{dims.Size, block}
}

func (l *List) draw() {
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
		max := axisMain(l.Axis, sz) + pos
		if max > mainc.Max {
			max = mainc.Max
		}
		min := pos
		if min < 0 {
			min = 0
		}
		r := image.Rectangle{
			Min: axisPoint(l.Axis, min, -ui.Inf),
			Max: axisPoint(l.Axis, max, ui.Inf),
		}
		l.ops.Begin()
		draw.OpClip{Path: draw.RectPath(r)}.Add(l.ops)
		ui.OpTransform{
			Transform: ui.Offset(toPointF(axisPoint(l.Axis, pos, cross))),
		}.Add(l.ops)
		child.block.Add(l.ops)
		l.ops.End().Add(l.ops)
		pos += axisMain(l.Axis, sz)
	}
	atStart := l.first == 0 && l.offset <= 0
	atEnd := l.first+len(l.children) == l.len && mainc.Max >= pos
	if atStart && l.scrollDir < 0 || atEnd && l.scrollDir > 0 {
		l.scroll.Stop()
	}
	l.size = axisPoint(l.Axis, mainc.Constrain(pos), maxCross)
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
