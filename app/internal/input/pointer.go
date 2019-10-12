// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"encoding/binary"
	"image"

	"gioui.org/f32"
	"gioui.org/internal/opconst"
	"gioui.org/internal/ops"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/op"
)

type pointerQueue struct {
	hitTree  []hitNode
	areas    []areaNode
	handlers map[event.Key]*pointerHandler
	pointers []pointerInfo
	reader   ops.Reader
	scratch  []event.Key
}

type hitNode struct {
	next int
	area int
	// Pass tracks the most recent PassOp mode.
	pass bool

	// For handler nodes.
	key event.Key
}

type pointerInfo struct {
	id       pointer.ID
	pressed  bool
	handlers []event.Key
}

type pointerHandler struct {
	area      int
	active    bool
	transform op.TransformOp
	wantsGrab bool
}

type areaOp struct {
	kind areaKind
	rect image.Rectangle
}

type areaNode struct {
	trans op.TransformOp
	next  int
	area  areaOp
}

type areaKind uint8

const (
	areaRect areaKind = iota
	areaEllipse
)

func (q *pointerQueue) collectHandlers(r *ops.Reader, events *handlerEvents, t op.TransformOp, area, node int, pass bool) {
	for encOp, ok := r.Decode(); ok; encOp, ok = r.Decode() {
		switch opconst.OpType(encOp.Data[0]) {
		case opconst.TypePush:
			q.collectHandlers(r, events, t, area, node, pass)
		case opconst.TypePop:
			return
		case opconst.TypePass:
			op := decodePassOp(encOp.Data)
			pass = op.Pass
		case opconst.TypeArea:
			var op areaOp
			op.Decode(encOp.Data)
			q.areas = append(q.areas, areaNode{trans: t, next: area, area: op})
			area = len(q.areas) - 1
			q.hitTree = append(q.hitTree, hitNode{
				next: node,
				area: area,
				pass: pass,
			})
			node = len(q.hitTree) - 1
		case opconst.TypeTransform:
			dop := ops.DecodeTransformOp(encOp.Data)
			t = t.Multiply(op.TransformOp(dop))
		case opconst.TypePointerInput:
			op := decodePointerInputOp(encOp.Data, encOp.Refs)
			q.hitTree = append(q.hitTree, hitNode{
				next: node,
				area: area,
				pass: pass,
				key:  op.Key,
			})
			node = len(q.hitTree) - 1
			h, ok := q.handlers[op.Key]
			if !ok {
				h = new(pointerHandler)
				q.handlers[op.Key] = h
				events.Set(op.Key, []event.Event{pointer.Event{Type: pointer.Cancel}})
			}
			h.active = true
			h.area = area
			h.transform = t
			h.wantsGrab = h.wantsGrab || op.Grab
		}
	}
}

func (q *pointerQueue) opHit(handlers *[]event.Key, pos f32.Point) {
	// Track whether we're passing through hits.
	pass := true
	idx := len(q.hitTree) - 1
	for idx >= 0 {
		n := &q.hitTree[idx]
		if !q.hit(n.area, pos) {
			idx--
			continue
		}
		pass = pass && n.pass
		if pass {
			idx--
		} else {
			idx = n.next
		}
		if n.key != nil {
			if _, exists := q.handlers[n.key]; exists {
				*handlers = append(*handlers, n.key)
			}

		}
	}
}

func (q *pointerQueue) hit(areaIdx int, p f32.Point) bool {
	for areaIdx != -1 {
		a := &q.areas[areaIdx]
		if !a.hit(p) {
			return false
		}
		areaIdx = a.next
	}
	return true
}

func (a *areaNode) hit(p f32.Point) bool {
	p = a.trans.Invert().Transform(p)
	return a.area.Hit(p)
}

func (q *pointerQueue) init() {
	if q.handlers == nil {
		q.handlers = make(map[event.Key]*pointerHandler)
	}
}

func (q *pointerQueue) Frame(root *op.Ops, events *handlerEvents) {
	q.init()
	for _, h := range q.handlers {
		// Reset handler.
		h.active = false
	}
	q.hitTree = q.hitTree[:0]
	q.areas = q.areas[:0]
	q.reader.Reset(root)
	q.collectHandlers(&q.reader, events, op.TransformOp{}, -1, -1, false)
	for k, h := range q.handlers {
		if !h.active {
			q.dropHandler(k, events)
			delete(q.handlers, k)
		}
	}
}

func (q *pointerQueue) dropHandler(k event.Key, events *handlerEvents) {
	events.Add(k, pointer.Event{Type: pointer.Cancel})
	q.handlers[k].wantsGrab = false
	for i := range q.pointers {
		p := &q.pointers[i]
		for i := len(p.handlers) - 1; i >= 0; i-- {
			if p.handlers[i] == k {
				p.handlers = append(p.handlers[:i], p.handlers[i+1:]...)
			}
		}
	}
}

func (q *pointerQueue) Push(e pointer.Event, events *handlerEvents) {
	q.init()
	if e.Type == pointer.Cancel {
		q.pointers = q.pointers[:0]
		for k := range q.handlers {
			q.dropHandler(k, events)
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
			q.dropHandler(k, events)
		}
	}
	if e.Type == pointer.Release {
		q.pointers = append(q.pointers[:pidx], q.pointers[pidx+1:]...)
	}
	for _, k := range p.handlers {
		h := q.handlers[k]
		e := e
		if p.pressed && len(p.handlers) == 1 {
			e.Priority = pointer.Grabbed
		}
		e.Hit = q.hit(h.area, e.Position)
		e.Position = h.transform.Invert().Transform(e.Position)
		events.Add(k, e)
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

func (op *areaOp) Decode(d []byte) {
	if opconst.OpType(d[0]) != opconst.TypeArea {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	rect := image.Rectangle{
		Min: image.Point{
			X: int(int32(bo.Uint32(d[2:]))),
			Y: int(int32(bo.Uint32(d[6:]))),
		},
		Max: image.Point{
			X: int(int32(bo.Uint32(d[10:]))),
			Y: int(int32(bo.Uint32(d[14:]))),
		},
	}
	*op = areaOp{
		kind: areaKind(d[1]),
		rect: rect,
	}
}

func (op *areaOp) Hit(pos f32.Point) bool {
	min := f32.Point{
		X: float32(op.rect.Min.X),
		Y: float32(op.rect.Min.Y),
	}
	pos = pos.Sub(min)
	size := op.rect.Size()
	switch op.kind {
	case areaRect:
		if 0 <= pos.X && pos.X < float32(size.X) &&
			0 <= pos.Y && pos.Y < float32(size.Y) {
			return true
		} else {
			return false
		}
	case areaEllipse:
		rx := float32(size.X) / 2
		ry := float32(size.Y) / 2
		rx2 := rx * rx
		ry2 := ry * ry
		xh := pos.X - rx
		yk := pos.Y - ry
		if xh*xh*ry2+yk*yk*rx2 <= rx2*ry2 {
			return true
		} else {
			return false
		}
	default:
		panic("invalid area kind")
	}
}

func decodePointerInputOp(d []byte, refs []interface{}) pointer.InputOp {
	if opconst.OpType(d[0]) != opconst.TypePointerInput {
		panic("invalid op")
	}
	return pointer.InputOp{
		Grab: d[1] != 0,
		Key:  refs[0].(event.Key),
	}
}

func decodePassOp(d []byte) pointer.PassOp {
	if opconst.OpType(d[0]) != opconst.TypePass {
		panic("invalid op")
	}
	return pointer.PassOp{
		Pass: d[1] != 0,
	}
}
