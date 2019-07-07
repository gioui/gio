// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"gioui.org/ui"
	"gioui.org/ui/internal/ops"
	"gioui.org/ui/key"
)

type keyQueue struct {
	focus    Key
	handlers map[Key]*keyHandler
	reader   ui.OpsReader
	state    key.TextInputState
}

type keyHandler struct {
	active bool
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
func (q *keyQueue) InputState() key.TextInputState {
	return q.state
}

func (q *keyQueue) Frame(root *ui.Ops, events handlerEvents) {
	if q.handlers == nil {
		q.handlers = make(map[Key]*keyHandler)
	}
	for _, h := range q.handlers {
		h.active = false
	}
	q.reader.Reset(root)
	focus, pri, hide := q.resolveFocus(events)
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
			events[q.focus] = append(events[q.focus], key.FocusEvent{Focus: false})
		}
		q.focus = focus
		if q.focus != nil {
			events[q.focus] = append(events[q.focus], key.FocusEvent{Focus: true})
		}
	}
	switch {
	case pri == priNewFocus:
		q.state = key.TextInputOpen
	case hide:
		q.state = key.TextInputClose
	case changed:
		q.state = key.TextInputFocus
	default:
		q.state = key.TextInputKeep
	}
}

func (q *keyQueue) Push(e Event, events handlerEvents) {
	if q.focus == nil {
		return
	}
	events[q.focus] = append(events[q.focus], e)
}

func (q *keyQueue) resolveFocus(events handlerEvents) (Key, listenerPriority, bool) {
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
			var op key.HandlerOp
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
			// Switch focus if higher priority or if focus requested.
			if newPri.replaces(pri) {
				k, pri = op.Key, newPri
			}
			h, ok := q.handlers[op.Key]
			if !ok {
				h = new(keyHandler)
				q.handlers[op.Key] = h
				// Reset the handler on (each) first appearance.
				events[op.Key] = []Event{key.FocusEvent{Focus: false}}
			}
			h.active = true
		case ops.TypeHideInput:
			hide = true
		case ops.TypePush:
			newK, newPri, h := q.resolveFocus(events)
			hide = hide || h
			if newPri.replaces(pri) {
				k, pri = newK, newPri
			}
		case ops.TypePop:
			break loop
		}
	}
	return k, pri, hide
}

func (p listenerPriority) replaces(p2 listenerPriority) bool {
	// Favor earliest default focus or latest requested focus.
	return p > p2 || p == p2 && p == priNewFocus
}
