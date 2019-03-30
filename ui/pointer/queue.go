// SPDX-License-Identifier: Unlicense OR MIT

package pointer

import (
	"gioui.org/ui/f32"
	"gioui.org/ui"
)

type Queue struct {
	// The root of the tree of ops relevant to pointer handling.
	root     ui.Op
	handlers map[Key]*handler
	pointers []pointerInfo
	scratch  []Key
}

type pointerInfo struct {
	id       ID
	pressed  bool
	handlers []Key
}

type handler struct {
	area      Area
	active    bool
	transform ui.Transform
	events    []Event
	wantsGrab bool
}

type childOp interface {
	ChildOp() ui.Op
}

func (q *Queue) collectHandlers(op ui.Op, t ui.Transform) ui.Op {
	switch op := op.(type) {
	case ui.Ops:
		var all ui.Ops
		for _, op := range op {
			if op := q.collectHandlers(op, t); op != nil {
				if ops, ok := op.(ui.Ops); ok {
					all = append(all, ops...)
				} else {
					all = append(all, op)
				}
			}
		}
		return all
	case ui.OpLayer:
		child := q.collectHandlers(op.ChildOp(), t)
		if child == nil {
			return nil
		}
		return ui.OpLayer{Op: child}
	case ui.OpTransform:
		return q.collectHandlers(op.ChildOp(), t.Mul(op.Transform))
	case OpHandler:
		h, ok := q.handlers[op.Key]
		if !ok {
			h = new(handler)
			q.handlers[op.Key] = h
		}
		h.area = op.Area
		h.transform = t
		h.wantsGrab = h.wantsGrab || op.Grab
		return op
	case childOp:
		return q.collectHandlers(op.ChildOp(), t)
	default:
		return nil
	}
}

func (q *Queue) opHit(handlers *[]Key, op ui.Op, pos f32.Point) (HitResult, bool) {
	if op == nil {
		return HitNone, false
	}
	switch op := op.(type) {
	case ui.Ops:
		hitRes := HitNone
		var layer bool
		for i := len(op) - 1; i >= 0; i-- {
			op := op[i]
			if _, ok := op.(ui.OpLayer); layer && ok {
				continue
			}
			res, l := q.opHit(handlers, op, pos)
			if res > hitRes {
				hitRes = res
			}
			layer = layer || l
		}
		return hitRes, layer
	case ui.OpLayer:
		res, layer := q.opHit(handlers, op.Op, pos)
		return res, layer || res == HitOpaque
	case OpHandler:
		h, ok := q.handlers[op.Key]
		if !ok {
			return HitNone, false
		}
		tpos := h.transform.InvTransform(pos)
		res := h.area(tpos)
		if res != HitNone {
			*handlers = append(*handlers, op.Key)
		}
		return res, false
	default:
		panic("unexpected op")
	}
}

func (q *Queue) init() {
	if q.handlers == nil {
		q.handlers = make(map[Key]*handler)
	}
}

func (q *Queue) Frame(op ui.Op) {
	q.init()
	for k, h := range q.handlers {
		if !h.active {
			q.dropHandler(k)
		} else {
			// Reset handler.
			h.events = h.events[:0]
		}
	}
	q.root = q.collectHandlers(op, ui.Transform{})
}

func (q *Queue) For(k Key) []Event {
	if k == nil {
		panic("nil handler")
	}
	q.init()
	h, ok := q.handlers[k]
	if !ok {
		h = new(handler)
		q.handlers[k] = h
	}
	if !h.active {
		h.active = true
		// Prepend a Cancel.
		h.events = append(h.events, Event{})
		copy(h.events[1:], h.events)
		h.events[0] = Event{Type: Cancel}
	}
	return h.events
}

func (q *Queue) dropHandler(k Key) {
	delete(q.handlers, k)
	for i := range q.pointers {
		p := &q.pointers[i]
		for i := len(p.handlers) - 1; i >= 0; i-- {
			if p.handlers[i] == k {
				p.handlers = append(p.handlers[:i], p.handlers[i+1:]...)
			}
		}
	}
}

func (q *Queue) Push(e Event) {
	q.init()
	if e.Type == Cancel {
		q.pointers = q.pointers[:0]
		for k := range q.handlers {
			q.dropHandler(k)
		}
		return
	}
	pidx := -1
	for i, p := range q.pointers {
		if p.id == e.PointerID {
			pidx = i
			break
		}
	}
	if pidx == -1 {
		q.pointers = append(q.pointers, pointerInfo{id: e.PointerID})
		pidx = len(q.pointers) - 1
	}
	p := &q.pointers[pidx]
	if !p.pressed && (e.Type == Move || e.Type == Press) {
		p.handlers, q.scratch = q.scratch[:0], p.handlers
		q.opHit(&p.handlers, q.root, e.Position)
		// Drop handlers no longer hit.
	loop:
		for _, h := range q.scratch {
			for _, h2 := range p.handlers {
				if h == h2 {
					continue loop
				}
			}
			q.dropHandler(h)
		}
		if e.Type == Press {
			p.pressed = true
		}
	}
	if p.pressed {
		// Resolve grabs.
		q.scratch = q.scratch[:0]
		for i, k := range p.handlers {
			h := q.handlers[k]
			if h.wantsGrab {
				q.scratch = append(q.scratch, p.handlers[:i]...)
				q.scratch = append(q.scratch, p.handlers[i+1:]...)
				break
			}
		}
		// Drop handlers that lost their grab.
		for _, k := range q.scratch {
			q.dropHandler(k)
		}
	}
	if e.Type == Release {
		q.pointers = append(q.pointers[:pidx], q.pointers[pidx+1:]...)
	}
	for i, k := range p.handlers {
		h := q.handlers[k]
		e := e
		switch {
		case p.pressed && len(p.handlers) == 1:
			e.Priority = Grabbed
		case i == 0:
			e.Priority = Foremost
		}
		e.Position = h.transform.InvTransform(e.Position)
		e.Hit = h.area(e.Position) != HitNone
		h.events = append(h.events, e)
		if e.Type == Release {
			// Release grab when the number of grabs reaches zero.
			grabs := 0
			for _, p := range q.pointers {
				if p.pressed && len(p.handlers) == 1 && p.handlers[0] == k {
					grabs++
				}
			}
			if grabs == 0 {
				h.wantsGrab = false
			}
		}
	}
}
