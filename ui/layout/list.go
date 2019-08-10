// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/ui"
	"gioui.org/ui/gesture"
	"gioui.org/ui/input"
	"gioui.org/ui/paint"
	"gioui.org/ui/pointer"
)

type scrollChild struct {
	size  image.Point
	macro ui.MacroOp
}

// List displays a subsection of a potentially infinitely
// large underlying list. List accepts user input to scroll
// the subsection.
type List struct {
	Axis Axis
	// Invert inverts a List so it is anchored from its end.
	Invert bool
	// Alignment is the cross axis alignment.
	Alignment Alignment

	// The distance scrolled since last call to Init.
	Distance int

	config    ui.Config
	ops       *ui.Ops
	queue     input.Queue
	macro     ui.MacroOp
	child     ui.MacroOp
	scroll    gesture.Scroll
	scrollDir int

	offset int
	first  int

	cs  Constraints
	len int

	maxSize  int
	children []scrollChild
	dir      iterationDir

	// Iterator state.
	index int
	more  bool
}

type iterationDir uint8

const (
	iterateNone iterationDir = iota
	iterateForward
	iterateBackward
)

const inf = 1e6

// Init prepares the list for iterating through its children with Next.
func (l *List) Init(cfg ui.Config, q input.Queue, ops *ui.Ops, cs Constraints, len int) {
	if l.more {
		panic("unfinished child")
	}
	l.config = cfg
	l.queue = q
	l.update()
	l.ops = ops
	l.dir = iterateNone
	l.maxSize = 0
	l.children = l.children[:0]
	l.cs = cs
	l.len = len
	l.more = true
	if l.first > len {
		l.first = len
	}
	l.macro.Record(ops)
	l.Next()
}

// Dragging reports whether the List is being dragged.
func (l *List) Dragging() bool {
	return l.scroll.Dragging()
}

func (l *List) update() {
	l.Distance = 0
	d := l.scroll.Scroll(l.config, l.queue, gesture.Axis(l.Axis))
	if l.Invert {
		d = -d
	}
	l.scrollDir = d
	l.Distance += d
	l.offset += d
}

// Next advances to the next child.
func (l *List) Next() {
	if !l.more {
		panic("end of list reached")
	}
	i, more := l.next()
	l.more = more
	if !more {
		return
	}
	if l.Invert {
		i = l.len - 1 - i
	}
	l.index = i
	l.child.Record(l.ops)
}

// Index is current child's position in the underlying list.
func (l *List) Index() int {
	return l.index
}

// Constraints is the constraints for the current child.
func (l *List) Constraints() Constraints {
	return axisConstraints(l.Axis, Constraint{Max: inf}, axisCrossConstraint(l.Axis, l.cs))
}

// More reports whether more children are needed.
func (l *List) More() bool {
	return l.more
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

// End the current child by specifying its dimensions.
func (l *List) End(dims Dimens) {
	l.child.Stop()
	child := scrollChild{dims.Size, l.child}
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

// Layout the List and return its dimensions.
func (l *List) Layout() Dimens {
	if l.more {
		panic("unfinished child")
	}
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
		switch l.Alignment {
		case End:
			cross = maxCross - axisCross(l.Axis, sz)
		case Middle:
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
			Min: axisPoint(l.Axis, min, -inf),
			Max: axisPoint(l.Axis, max, inf),
		}
		var stack ui.StackOp
		stack.Push(ops)
		paint.RectClip(r).Add(ops)
		ui.TransformOp{}.Offset(toPointF(axisPoint(l.Axis, transPos, cross))).Add(ops)
		child.macro.Add(ops)
		stack.Pop()
		pos += childSize
	}
	atStart := l.first == 0 && l.offset <= 0
	atEnd := l.first+len(l.children) == l.len && mainc.Max >= pos
	if atStart && l.scrollDir < 0 || atEnd && l.scrollDir > 0 {
		l.scroll.Stop()
	}
	dims := axisPoint(l.Axis, mainc.Constrain(pos), maxCross)
	l.macro.Stop()
	pointer.RectAreaOp{Rect: image.Rectangle{Max: dims}}.Add(ops)
	l.scroll.Add(ops)
	l.macro.Add(ops)
	return Dimens{Size: dims}
}
