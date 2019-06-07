// SPDX-License-Identifier: Unlicense OR MIT

package key

import (
	"gioui.org/ui"
	"gioui.org/ui/internal/ops"
)

type Queue struct {
	focus    Key
	handlers map[Key]*handler
	reader   ui.OpsReader
	state    TextInputState
}

type handler struct {
	active bool
	events []Event
}

type listenerPriority uint8

const (
	priNone listenerPriority = iota
	priDefault
	priCurrentFocus
	priNewFocus
)

// InputState returns the last text input state as
// determined in Frame.
func (q *Queue) InputState() TextInputState {
	return q.state
}

func (q *Queue) Frame(root *ui.Ops) {
	if q.handlers == nil {
		q.handlers = make(map[Key]*handler)
	}
	for _, h := range q.handlers {
		h.active = false
		h.events = h.events[:0]
	}
	q.reader.Reset(root)
	focus, pri, hide := q.resolveFocus()
	for k, h := range q.handlers {
		if !h.active {
			delete(q.handlers, k)
			if q.focus == k {
				q.focus = nil
			}
		}
	}
	changed := focus != nil && focus != q.focus
	if focus != q.focus {
		if q.focus != nil {
			if h, ok := q.handlers[q.focus]; ok {
				h.events = append(h.events, Focus{Focus: false})
			}
		}
		q.focus = focus
		if q.focus != nil {
			// A new focus always exists in the handler map.
			h := q.handlers[q.focus]
			h.events = append(h.events, Focus{Focus: true})
		}
	}
	switch {
	case pri == priNewFocus:
		q.state = TextInputOpen
	case hide:
		q.state = TextInputClosed
	case changed:
		q.state = TextInputFocus
	default:
		q.state = TextInputKeep
	}
}

func (q *Queue) Push(e Event) {
	if q.focus == nil {
		return
	}
	h := q.handlers[q.focus]
	h.events = append(h.events, e)
}

func (q *Queue) For(k Key) []Event {
	h := q.handlers[k]
	if h == nil {
		return nil
	}
	return h.events
}

func (q *Queue) resolveFocus() (Key, listenerPriority, bool) {
	var k Key
	var pri listenerPriority
	var hide bool
loop:
	for {
		encOp, ok := q.reader.Decode()
		if !ok {
			break
		}
		switch ops.OpType(encOp.Data[0]) {
		case ops.TypeKeyHandler:
			var op OpHandler
			op.Decode(encOp.Data, encOp.Refs)
			var newPri listenerPriority
			switch {
			case op.Focus:
				newPri = priNewFocus
			case op.Key == q.focus:
				newPri = priCurrentFocus
			default:
				newPri = priDefault
			}
			if newPri >= pri {
				k, pri = op.Key, newPri
			}
			h, ok := q.handlers[op.Key]
			if !ok {
				h = &handler{
					// Reset the handler on (each) first appearance.
					events: []Event{Focus{Focus: false}},
				}
				q.handlers[op.Key] = h
			}
			h.active = true
		case ops.TypeHideInput:
			hide = true
		case ops.TypePush:
			newK, newPri, h := q.resolveFocus()
			hide = hide || h
			if newPri >= pri {
				k, pri = newK, newPri
			}
		case ops.TypePop:
			break loop
		}
	}
	return k, pri, hide
}
