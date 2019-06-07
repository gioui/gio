// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"gioui.org/ui"
	"gioui.org/ui/f32"
	"gioui.org/ui/internal/ops"
	"gioui.org/ui/pointer"
)

type pointerQueue struct {
	hitTree  []hitNode
	handlers map[Key]*pointerHandler
	pointers []pointerInfo
	reader   ui.OpsReader
	scratch  []Key
	areas    areaStack
}

type hitNode struct {
	// The layer depth.
	level int
	// The handler, or nil for a layer.
	key Key
}

type pointerInfo struct {
	id       pointer.ID
	pressed  bool
	handlers []Key
}

type pointerHandler struct {
	area      areaIntersection
	active    bool
	transform ui.Transform
	wantsGrab bool
}

type area struct {
	trans ui.Transform
	area  pointer.OpArea
}

type areaIntersection []area

type areaStack struct {
	stack   []int
	areas   []area
	backing []area
}

func (q *pointerQueue) collectHandlers(r *ui.OpsReader, t ui.Transform, layer int, events handlerEvents) {
	for {
		encOp, ok := r.Decode()
		if !ok {
			return
		}
		switch ops.OpType(encOp.Data[0]) {
		case ops.TypePush:
			q.areas.push()
			q.collectHandlers(r, t, layer, events)
		case ops.TypePop:
			q.areas.pop()
			return
		case ops.TypeLayer:
			layer++
			q.hitTree = append(q.hitTree, hitNode{level: layer})
		case ops.TypeArea:
			var op pointer.OpArea
			op.Decode(encOp.Data)
			q.areas.add(t, op)
		case ops.TypeTransform:
			var op ui.OpTransform
			op.Decode(encOp.Data)
			t = t.Mul(op.Transform)
		case ops.TypePointerHandler:
			var op pointer.OpHandler
			op.Decode(encOp.Data, encOp.Refs)
			q.hitTree = append(q.hitTree, hitNode{level: layer, key: op.Key})
			h, ok := q.handlers[op.Key]
			if !ok {
				h = new(pointerHandler)
				q.handlers[op.Key] = h
				events[op.Key] = []Event{pointer.Event{Type: pointer.Cancel}}
			}
			h.active = true
			h.area = q.areas.intersection()
			h.transform = t
			h.wantsGrab = h.wantsGrab || op.Grab
		}
	}
}

func (q *pointerQueue) opHit(handlers *[]Key, pos f32.Point) {
	level := 1 << 30
	opaque := false
	for i := len(q.hitTree) - 1; i >= 0; i-- {
		n := q.hitTree[i]
		if n.key == nil {
			// Layer
			if opaque {
				opaque = false
				// Skip sibling handlers.
				level = n.level - 1
			}
		} else if n.level <= level {
			// Handler
			h, ok := q.handlers[n.key]
			if !ok {
				continue
			}
			res := h.area.hit(pos)
			opaque = opaque || res == pointer.HitOpaque
			if res != pointer.HitNone {
				*handlers = append(*handlers, n.key)
			}
		}
	}
}

func (q *pointerQueue) init() {
	if q.handlers == nil {
		q.handlers = make(map[Key]*pointerHandler)
	}
}

func (q *pointerQueue) Frame(root *ui.Ops, events handlerEvents) {
	q.init()
	for _, h := range q.handlers {
		// Reset handler.
		h.active = false
	}
	q.hitTree = q.hitTree[:0]
	q.areas.reset()
	q.reader.Reset(root)
	q.collectHandlers(&q.reader, ui.Transform{}, 0, events)
	for k, h := range q.handlers {
		if !h.active {
			q.dropHandler(k)
		}
	}
}

func (q *pointerQueue) dropHandler(k Key) {
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

func (q *pointerQueue) Push(e pointer.Event, events handlerEvents) {
	q.init()
	if e.Type == pointer.Cancel {
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
	if !p.pressed && (e.Type == pointer.Move || e.Type == pointer.Press) {
		p.handlers, q.scratch = q.scratch[:0], p.handlers
		q.opHit(&p.handlers, e.Position)
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
		if e.Type == pointer.Press {
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
	if e.Type == pointer.Release {
		q.pointers = append(q.pointers[:pidx], q.pointers[pidx+1:]...)
	}
	for i, k := range p.handlers {
		h := q.handlers[k]
		e := e
		switch {
		case p.pressed && len(p.handlers) == 1:
			e.Priority = pointer.Grabbed
		case i == 0:
			e.Priority = pointer.Foremost
		}
		e.Hit = h.area.hit(e.Position) != pointer.HitNone
		e.Position = h.transform.InvTransform(e.Position)
		events[k] = append(events[k], e)
		if e.Type == pointer.Release {
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

func (a areaIntersection) hit(p f32.Point) pointer.HitResult {
	res := pointer.HitNone
	for _, area := range a {
		tp := area.trans.InvTransform(p)
		res = area.area.Hit(tp)
		if res == pointer.HitNone {
			break
		}
	}
	return res
}

func (s *areaStack) add(t ui.Transform, a pointer.OpArea) {
	s.areas = append(s.areas, area{t, a})
}

func (s *areaStack) push() {
	s.stack = append(s.stack, len(s.areas))
}

func (s *areaStack) pop() {
	off := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	s.areas = s.areas[:off]
}

func (s *areaStack) intersection() areaIntersection {
	off := len(s.backing)
	s.backing = append(s.backing, s.areas...)
	return areaIntersection(s.backing[off:len(s.backing):len(s.backing)])
}

func (a *areaStack) reset() {
	a.areas = a.areas[:0]
	a.stack = a.stack[:0]
	a.backing = a.backing[:0]
}
