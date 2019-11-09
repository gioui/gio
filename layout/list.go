// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/op"
	"gioui.org/op/clip"
)

type scrollChild struct {
	size  image.Point
	macro op.MacroOp
}

// List displays a subsection of a potentially infinitely
// large underlying list. List accepts user input to scroll
// the subsection.
type List struct {
	Axis Axis
	// ScrollToEnd instructs the list to stay scrolled to the far end position
	// once reahed. A List with ScrollToEnd enabled also align its content to
	// the end.
	ScrollToEnd bool
	// Alignment is the cross axis alignment of list elements.
	Alignment Alignment

	// beforeEnd tracks whether the List position is before
	// the very end.
	beforeEnd bool

	ctx         *Context
	macro       op.MacroOp
	child       op.MacroOp
	scroll      gesture.Scroll
	scrollDelta int

	// first is the index of the first visible child.
	first int
	// offset is the signed distance from the top edge
	// to the child with index first.
	offset int

	len int

	// maxSize is the total size of visible children.
	maxSize  int
	children []scrollChild
	dir      iterationDir
}

// ListElement is a function that computes the dimensions of
// a list element.
type ListElement func(index int)

type iterationDir uint8

const (
	iterateNone iterationDir = iota
	iterateForward
	iterateBackward
)

const inf = 1e6

// init prepares the list for iterating through its children with next.
func (l *List) init(gtx *Context, len int) {
	if l.more() {
		panic("unfinished child")
	}
	l.ctx = gtx
	l.maxSize = 0
	l.children = l.children[:0]
	l.len = len
	l.update()
	if l.scrollToEnd() {
		l.offset = 0
		l.first = len
	}
	if l.first > len {
		l.offset = 0
		l.first = len
	}
	l.macro.Record(gtx.Ops)
	l.next()
}

// Layout the List.
func (l *List) Layout(gtx *Context, len int, w ListElement) {
	for l.init(gtx, len); l.more(); l.next() {
		cs := axisConstraints(l.Axis, Constraint{Max: inf}, axisCrossConstraint(l.Axis, l.ctx.Constraints))
		i := l.index()
		l.end(ctxLayout(gtx, cs, func() {
			w(i)
		}))
	}
	gtx.Dimensions = l.layout()
}

func (l *List) scrollToEnd() bool {
	return l.ScrollToEnd && !l.beforeEnd
}

// Dragging reports whether the List is being dragged.
func (l *List) Dragging() bool {
	return l.scroll.State() == gesture.StateDragging
}

func (l *List) update() {
	d := l.scroll.Scroll(l.ctx.Config, l.ctx.Queue, l.ctx.Now(), gesture.Axis(l.Axis))
	l.scrollDelta = d
	l.offset += d
}

// next advances to the next child.
func (l *List) next() {
	l.dir = l.nextDir()
	// The user scroll offset is applied after scrolling to
	// list end.
	if l.scrollToEnd() && !l.more() && l.scrollDelta < 0 {
		l.beforeEnd = true
		l.offset += l.scrollDelta
		l.dir = l.nextDir()
	}
	if l.more() {
		l.child.Record(l.ctx.Ops)
	}
}

// index is current child's position in the underlying list.
func (l *List) index() int {
	switch l.dir {
	case iterateBackward:
		return l.first - 1
	case iterateForward:
		return l.first + len(l.children)
	default:
		panic("Index called before Next")
	}
}

// more reports whether more children are needed.
func (l *List) more() bool {
	return l.dir != iterateNone
}

func (l *List) nextDir() iterationDir {
	vsize := axisMainConstraint(l.Axis, l.ctx.Constraints).Max
	last := l.first + len(l.children)
	// Clamp offset.
	if l.maxSize-l.offset < vsize && last == l.len {
		l.offset = l.maxSize - vsize
	}
	if l.offset < 0 && l.first == 0 {
		l.offset = 0
	}
	switch {
	case len(l.children) == l.len:
		return iterateNone
	case l.maxSize-l.offset < vsize:
		return iterateForward
	case l.offset < 0:
		return iterateBackward
	}
	return iterateNone
}

// End the current child by specifying its dimensions.
func (l *List) end(dims Dimensions) {
	l.child.Stop()
	child := scrollChild{dims.Size, l.child}
	mainSize := axisMain(l.Axis, child.size)
	l.maxSize += mainSize
	switch l.dir {
	case iterateForward:
		l.children = append(l.children, child)
	case iterateBackward:
		l.children = append([]scrollChild{child}, l.children...)
		l.first--
		l.offset += mainSize
	default:
		panic("call Next before End")
	}
	l.dir = iterateNone
}

// Layout the List and return its dimensions.
func (l *List) layout() Dimensions {
	if l.more() {
		panic("unfinished child")
	}
	mainc := axisMainConstraint(l.Axis, l.ctx.Constraints)
	children := l.children
	// Skip invisible children
	for len(children) > 0 {
		sz := children[0].size
		mainSize := axisMain(l.Axis, sz)
		if l.offset <= mainSize {
			break
		}
		l.first++
		l.offset -= mainSize
		children = children[1:]
	}
	size := -l.offset
	var maxCross int
	for i, child := range children {
		sz := child.size
		if c := axisCross(l.Axis, sz); c > maxCross {
			maxCross = c
		}
		size += axisMain(l.Axis, sz)
		if size >= mainc.Max {
			children = children[:i+1]
			break
		}
	}
	ops := l.ctx.Ops
	pos := -l.offset
	// ScrollToEnd lists lists are end aligned.
	if space := mainc.Max - size; l.ScrollToEnd && space > 0 {
		pos += space
	}
	for _, child := range children {
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
		r := image.Rectangle{
			Min: axisPoint(l.Axis, min, -inf),
			Max: axisPoint(l.Axis, max, inf),
		}
		var stack op.StackOp
		stack.Push(ops)
		clip.Rect(r).Add(ops)
		op.TransformOp{}.Offset(toPointF(axisPoint(l.Axis, pos, cross))).Add(ops)
		child.macro.Add(ops)
		stack.Pop()
		pos += childSize
	}
	atStart := l.first == 0 && l.offset <= 0
	atEnd := l.first+len(children) == l.len && mainc.Max >= pos
	if atStart && l.scrollDelta < 0 || atEnd && l.scrollDelta > 0 {
		l.scroll.Stop()
	}
	l.beforeEnd = !atEnd
	dims := axisPoint(l.Axis, mainc.Constrain(pos), maxCross)
	l.macro.Stop()
	pointer.RectAreaOp{Rect: image.Rectangle{Max: dims}}.Add(ops)
	l.scroll.Add(ops)
	l.macro.Add(ops)
	return Dimensions{Size: dims}
}
