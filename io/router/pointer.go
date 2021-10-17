// SPDX-License-Identifier: Unlicense OR MIT

package router

import (
	"encoding/binary"
	"image"

	"gioui.org/f32"
	"gioui.org/internal/ops"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op"
)

type pointerQueue struct {
	hitTree  []hitNode
	areas    []areaNode
	cursors  []cursorNode
	cursor   pointer.CursorName
	handlers map[event.Tag]*pointerHandler
	pointers []pointerInfo
	reader   ops.Reader

	nodeStack  []int
	transStack []f32.Affine2D
	// states holds the storage for save/restore ops.
	states  []f32.Affine2D
	scratch []event.Tag
}

type hitNode struct {
	next int
	area int

	// For handler nodes.
	tag event.Tag
}

type cursorNode struct {
	name pointer.CursorName
	area int
}

type pointerInfo struct {
	id       pointer.ID
	pressed  bool
	handlers []event.Tag
	// last tracks the last pointer event received,
	// used while processing frame events.
	last pointer.Event

	// entered tracks the tags that contain the pointer.
	entered []event.Tag
}

type pointerHandler struct {
	area      int
	active    bool
	wantsGrab bool
	types     pointer.Type
	// min and max horizontal/vertical scroll
	scrollRange image.Rectangle
}

type areaOp struct {
	kind areaKind
	rect f32.Rectangle
}

type areaNode struct {
	trans f32.Affine2D
	next  int
	area  areaOp
	pass  bool
}

type areaKind uint8

// collectState represents the state for collectHandlers
type collectState struct {
	t    f32.Affine2D
	node int
	pass int
}

const (
	areaRect areaKind = iota
	areaEllipse
)

func (q *pointerQueue) save(id int, state f32.Affine2D) {
	if extra := id - len(q.states) + 1; extra > 0 {
		q.states = append(q.states, make([]f32.Affine2D, extra)...)
	}
	q.states[id] = state
}

func (q *pointerQueue) collectHandlers(r *ops.Reader, events *handlerEvents) {
	var state collectState
	reset := func() {
		state = collectState{
			node: -1,
		}
	}
	reset()
	for encOp, ok := r.Decode(); ok; encOp, ok = r.Decode() {
		switch ops.OpType(encOp.Data[0]) {
		case ops.TypeSave:
			id := ops.DecodeSave(encOp.Data)
			q.save(id, state.t)
		case ops.TypeLoad:
			reset()
			id := ops.DecodeLoad(encOp.Data)
			state.t = q.states[id]
		case ops.TypeArea:
			var op areaOp
			op.Decode(encOp.Data)
			area := -1
			if i := state.node; i != -1 {
				n := q.hitTree[i]
				area = n.area
			}
			q.areas = append(q.areas, areaNode{trans: state.t, next: area, area: op, pass: state.pass > 0})
			q.nodeStack = append(q.nodeStack, state.node)
			q.hitTree = append(q.hitTree, hitNode{
				next: state.node,
				area: len(q.areas) - 1,
			})
			state.node = len(q.hitTree) - 1
		case ops.TypePopArea:
			n := len(q.nodeStack)
			state.node = q.nodeStack[n-1]
			q.nodeStack = q.nodeStack[:n-1]
		case ops.TypePass:
			state.pass++
		case ops.TypePopPass:
			state.pass--
		case ops.TypeTransform:
			dop, push := ops.DecodeTransform(encOp.Data)
			if push {
				q.transStack = append(q.transStack, state.t)
			}
			state.t = state.t.Mul(dop)
		case ops.TypePopTransform:
			n := len(q.transStack)
			state.t = q.transStack[n-1]
			q.transStack = q.transStack[:n-1]
		case ops.TypePointerInput:
			op := pointer.InputOp{
				Tag:   encOp.Refs[0].(event.Tag),
				Grab:  encOp.Data[1] != 0,
				Types: pointer.Type(encOp.Data[2]),
			}
			area := -1
			if i := state.node; i != -1 {
				n := q.hitTree[i]
				area = n.area
			}
			q.hitTree = append(q.hitTree, hitNode{
				next: state.node,
				area: area,
				tag:  op.Tag,
			})
			state.node = len(q.hitTree) - 1
			h, ok := q.handlers[op.Tag]
			if !ok {
				h = new(pointerHandler)
				q.handlers[op.Tag] = h
				// Cancel handlers on (each) first appearance, but don't
				// trigger redraw.
				events.AddNoRedraw(op.Tag, pointer.Event{Type: pointer.Cancel})
			}
			h.active = true
			h.area = area
			h.wantsGrab = h.wantsGrab || op.Grab
			h.types = h.types | op.Types
			bo := binary.LittleEndian.Uint32
			h.scrollRange = image.Rectangle{
				Min: image.Point{
					X: int(int32(bo(encOp.Data[3:]))),
					Y: int(int32(bo(encOp.Data[7:]))),
				},
				Max: image.Point{
					X: int(int32(bo(encOp.Data[11:]))),
					Y: int(int32(bo(encOp.Data[15:]))),
				},
			}
		case ops.TypeCursor:
			q.cursors = append(q.cursors, cursorNode{
				name: encOp.Refs[0].(pointer.CursorName),
				area: len(q.areas) - 1,
			})
		}
	}
}

func (q *pointerQueue) opHit(handlers *[]event.Tag, pos f32.Point) {
	// Track whether we're passing through hits.
	pass := true
	idx := len(q.hitTree) - 1
	for idx >= 0 {
		n := &q.hitTree[idx]
		hit, areaPass := q.hit(n.area, pos)
		if !hit {
			idx--
			continue
		}
		pass = pass && areaPass
		if pass {
			idx--
		} else {
			idx = n.next
		}
		if n.tag != nil {
			if _, exists := q.handlers[n.tag]; exists {
				*handlers = append(*handlers, n.tag)
			}
		}
	}
}

func (q *pointerQueue) invTransform(areaIdx int, p f32.Point) f32.Point {
	if areaIdx == -1 {
		return p
	}
	return q.areas[areaIdx].trans.Invert().Transform(p)
}

func (q *pointerQueue) hit(areaIdx int, p f32.Point) (bool, bool) {
	pass := false
	for areaIdx != -1 {
		a := &q.areas[areaIdx]
		p := a.trans.Invert().Transform(p)
		if !a.area.Hit(p) {
			return false, false
		}
		areaIdx = a.next
		pass = pass || a.pass
	}
	return true, pass
}

func (q *pointerQueue) reset() {
	if q.handlers == nil {
		q.handlers = make(map[event.Tag]*pointerHandler)
	}
}

func (q *pointerQueue) Frame(root *op.Ops, events *handlerEvents) {
	q.reset()
	for _, h := range q.handlers {
		// Reset handler.
		h.active = false
		h.wantsGrab = false
		h.types = 0
	}
	q.hitTree = q.hitTree[:0]
	q.areas = q.areas[:0]
	q.nodeStack = q.nodeStack[:0]
	q.transStack = q.transStack[:0]
	q.cursors = q.cursors[:0]
	q.reader.Reset(&root.Internal)
	q.collectHandlers(&q.reader, events)
	for k, h := range q.handlers {
		if !h.active {
			q.dropHandler(nil, k)
			delete(q.handlers, k)
		}
		if h.wantsGrab {
			for _, p := range q.pointers {
				if !p.pressed {
					continue
				}
				for i, k2 := range p.handlers {
					if k2 == k {
						// Drop other handlers that lost their grab.
						dropped := make([]event.Tag, 0, len(p.handlers)-1)
						dropped = append(dropped, p.handlers[:i]...)
						dropped = append(dropped, p.handlers[i+1:]...)
						for _, tag := range dropped {
							q.dropHandler(events, tag)
						}
						break
					}
				}
			}
		}
	}
	for i := range q.pointers {
		p := &q.pointers[i]
		q.deliverEnterLeaveEvents(p, events, p.last)
	}
}

func (q *pointerQueue) dropHandler(events *handlerEvents, tag event.Tag) {
	if events != nil {
		events.Add(tag, pointer.Event{Type: pointer.Cancel})
	}
	for i := range q.pointers {
		p := &q.pointers[i]
		for i := len(p.handlers) - 1; i >= 0; i-- {
			if p.handlers[i] == tag {
				p.handlers = append(p.handlers[:i], p.handlers[i+1:]...)
			}
		}
		for i := len(p.entered) - 1; i >= 0; i-- {
			if p.entered[i] == tag {
				p.entered = append(p.entered[:i], p.entered[i+1:]...)
			}
		}
	}
}

// pointerOf returns the pointerInfo index corresponding to the pointer in e.
func (q *pointerQueue) pointerOf(e pointer.Event) int {
	for i, p := range q.pointers {
		if p.id == e.PointerID {
			return i
		}
	}
	q.pointers = append(q.pointers, pointerInfo{id: e.PointerID})
	return len(q.pointers) - 1
}

func (q *pointerQueue) Push(e pointer.Event, events *handlerEvents) {
	q.reset()
	if e.Type == pointer.Cancel {
		q.pointers = q.pointers[:0]
		for k := range q.handlers {
			q.dropHandler(events, k)
		}
		return
	}
	pidx := q.pointerOf(e)
	p := &q.pointers[pidx]
	p.last = e

	switch e.Type {
	case pointer.Press:
		q.deliverEnterLeaveEvents(p, events, e)
		p.pressed = true
		q.deliverEvent(p, events, e)
	case pointer.Move:
		if p.pressed {
			e.Type = pointer.Drag
		}
		q.deliverEnterLeaveEvents(p, events, e)
		q.deliverEvent(p, events, e)
	case pointer.Release:
		q.deliverEvent(p, events, e)
		p.pressed = false
		q.deliverEnterLeaveEvents(p, events, e)
	case pointer.Scroll:
		q.deliverEnterLeaveEvents(p, events, e)
		q.deliverScrollEvent(p, events, e)
	default:
		panic("unsupported pointer event type")
	}

	if !p.pressed && len(p.entered) == 0 {
		// No longer need to track pointer.
		q.pointers = append(q.pointers[:pidx], q.pointers[pidx+1:]...)
	}
}

func (q *pointerQueue) deliverEvent(p *pointerInfo, events *handlerEvents, e pointer.Event) {
	foremost := true
	if p.pressed && len(p.handlers) == 1 {
		e.Priority = pointer.Grabbed
		foremost = false
	}
	for _, k := range p.handlers {
		h := q.handlers[k]
		if e.Type&h.types == 0 {
			continue
		}
		e := e
		if foremost {
			foremost = false
			e.Priority = pointer.Foremost
		}
		e.Position = q.invTransform(h.area, e.Position)
		events.Add(k, e)
	}
}

func (q *pointerQueue) deliverScrollEvent(p *pointerInfo, events *handlerEvents, e pointer.Event) {
	foremost := true
	if p.pressed && len(p.handlers) == 1 {
		e.Priority = pointer.Grabbed
		foremost = false
	}
	var sx, sy = e.Scroll.X, e.Scroll.Y
	for _, k := range p.handlers {
		if sx == 0 && sy == 0 {
			return
		}
		h := q.handlers[k]
		// Distribute the scroll to the handler based on its ScrollRange.
		sx, e.Scroll.X = setScrollEvent(sx, h.scrollRange.Min.X, h.scrollRange.Max.X)
		sy, e.Scroll.Y = setScrollEvent(sy, h.scrollRange.Min.Y, h.scrollRange.Max.Y)
		e := e
		if foremost {
			foremost = false
			e.Priority = pointer.Foremost
		}
		e.Position = q.invTransform(h.area, e.Position)
		events.Add(k, e)
	}
}

func (q *pointerQueue) deliverEnterLeaveEvents(p *pointerInfo, events *handlerEvents, e pointer.Event) {
	q.scratch = q.scratch[:0]
	q.opHit(&q.scratch, e.Position)
	if p.pressed {
		// Filter out non-participating handlers.
		for i := len(q.scratch) - 1; i >= 0; i-- {
			if _, found := searchTag(p.handlers, q.scratch[i]); !found {
				q.scratch = append(q.scratch[:i], q.scratch[i+1:]...)
			}
		}
	} else {
		p.handlers = append(p.handlers[:0], q.scratch...)
	}
	hits := q.scratch
	if e.Source != pointer.Mouse && !p.pressed && e.Type != pointer.Press {
		// Consider non-mouse pointers leaving when they're released.
		hits = nil
	}
	// Deliver Leave events.
	for _, k := range p.entered {
		if _, found := searchTag(hits, k); found {
			continue
		}
		h := q.handlers[k]
		e.Type = pointer.Leave

		if e.Type&h.types != 0 {
			e.Position = q.invTransform(h.area, e.Position)
			events.Add(k, e)
		}
	}
	// Deliver Enter events and update cursor.
	q.cursor = pointer.CursorDefault
	for _, k := range hits {
		h := q.handlers[k]
		for i := len(q.cursors) - 1; i >= 0; i-- {
			if c := q.cursors[i]; c.area == h.area {
				q.cursor = c.name
				break
			}
		}
		if _, found := searchTag(p.entered, k); found {
			continue
		}
		e.Type = pointer.Enter

		if e.Type&h.types != 0 {
			e.Position = q.invTransform(h.area, e.Position)
			events.Add(k, e)
		}
	}
	p.entered = append(p.entered[:0], hits...)
}

func searchTag(tags []event.Tag, tag event.Tag) (int, bool) {
	for i, t := range tags {
		if t == tag {
			return i, true
		}
	}
	return 0, false
}

func opDecodeFloat32(d []byte) float32 {
	return float32(int32(binary.LittleEndian.Uint32(d)))
}

func (op *areaOp) Decode(d []byte) {
	if ops.OpType(d[0]) != ops.TypeArea {
		panic("invalid op")
	}
	rect := f32.Rectangle{
		Min: f32.Point{
			X: opDecodeFloat32(d[2:]),
			Y: opDecodeFloat32(d[6:]),
		},
		Max: f32.Point{
			X: opDecodeFloat32(d[10:]),
			Y: opDecodeFloat32(d[14:]),
		},
	}
	*op = areaOp{
		kind: areaKind(d[1]),
		rect: rect,
	}
}

func (op *areaOp) Hit(pos f32.Point) bool {
	pos = pos.Sub(op.rect.Min)
	size := op.rect.Size()
	switch op.kind {
	case areaRect:
		return 0 <= pos.X && pos.X < size.X &&
			0 <= pos.Y && pos.Y < size.Y
	case areaEllipse:
		rx := size.X / 2
		ry := size.Y / 2
		xh := pos.X - rx
		yk := pos.Y - ry
		// The ellipse function works in all cases because
		// 0/0 is not <= 1.
		return (xh*xh)/(rx*rx)+(yk*yk)/(ry*ry) <= 1
	default:
		panic("invalid area kind")
	}
}

func setScrollEvent(scroll float32, min, max int) (left, scrolled float32) {
	if v := float32(max); scroll > v {
		return scroll - v, v
	}
	if v := float32(min); scroll < v {
		return scroll - v, v
	}
	return 0, scroll
}
