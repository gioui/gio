// SPDX-License-Identifier: Unlicense OR MIT

package pointer

import (
	"gioui.org/ui"
	"gioui.org/ui/f32"
	"gioui.org/ui/internal/ops"
)

type Queue struct {
	hitTree  []hitNode
	handlers map[Key]*handler
	pointers []pointerInfo
	reader   ops.Reader
	scratch  []Key
}

type hitNode struct {
	// The layer depth.
	level int
	// The handler, or nil for a layer.
	key Key
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

func (q *Queue) collectHandlers(r *ops.Reader, t ui.Transform, layer int) {
	for {
		data, ok := r.Decode()
		if !ok {
			return
		}
		switch ops.OpType(data[0]) {
		case ops.TypePush:
			q.collectHandlers(r, t, layer)
		case ops.TypePop:
			return
		case ops.TypeLayer:
			layer++
			q.hitTree = append(q.hitTree, hitNode{level: layer})
		case ops.TypeTransform:
			var op ui.OpTransform
			op.Decode(data)
			t = t.Mul(op.Transform)
		case ops.TypePointerHandler:
			var op OpHandler
			op.Decode(data, r.Refs)
			q.hitTree = append(q.hitTree, hitNode{level: layer, key: op.Key})
			h, ok := q.handlers[op.Key]
			if !ok {
				h = new(handler)
				q.handlers[op.Key] = h
			}
			h.area = op.Area
			h.transform = t
			h.wantsGrab = h.wantsGrab || op.Grab
		}
	}
}

func (q *Queue) opHit(handlers *[]Key, pos f32.Point) {
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
			tpos := h.transform.InvTransform(pos)
			res := h.area.Hit(tpos)
			opaque = opaque || res == HitOpaque
			if res != HitNone {
				*handlers = append(*handlers, n.key)
			}
		}
	}
}

func (q *Queue) init() {
	if q.handlers == nil {
		q.handlers = make(map[Key]*handler)
	}
}

func (q *Queue) Frame(root *ui.Ops) {
	q.init()
	for k, h := range q.handlers {
		if !h.active {
			q.dropHandler(k)
		} else {
			// Reset handler.
			h.events = h.events[:0]
		}
	}
	q.hitTree = q.hitTree[:0]
	q.reader.Reset(root.Data(), root.Refs())
	q.collectHandlers(&q.reader, ui.Transform{}, 0)
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
		e.Hit = h.area.Hit(e.Position) != HitNone
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
