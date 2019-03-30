// SPDX-License-Identifier: Unlicense OR MIT

package key

import (
	"gioui.org/ui"
)

type Queue struct {
	focus    Key
	events   []Event
	handlers map[Key]bool
}

type listenerPriority uint8

const (
	priNone listenerPriority = iota
	priDefault
	priCurrentFocus
	priNewFocus
)

func (q *Queue) Frame(op ui.Op) TextInputState {
	q.events = q.events[:0]
	f, pri, hide := resolveFocus(op, q.focus)
	changed := f != nil && f != q.focus
	for k, active := range q.handlers {
		if !active || changed {
			delete(q.handlers, k)
		} else {
			q.handlers[k] = false
		}
	}
	q.focus = f
	switch {
	case pri == priNewFocus:
		return TextInputOpen
	case hide:
		return TextInputClosed
	case changed:
		return TextInputFocus
	default:
		return TextInputKeep
	}
}

func (q *Queue) Push(e Event) {
	q.events = append(q.events, e)
}

func (q *Queue) For(k Key) []Event {
	if q.handlers == nil {
		q.handlers = make(map[Key]bool)
	}
	_, exists := q.handlers[k]
	q.handlers[k] = true
	if !exists {
		if k == q.focus {
			// Prepend focus event.
			q.events = append(q.events, nil)
			copy(q.events[1:], q.events)
			q.events[0] = Focus{Focus: true}
		} else {
			return []Event{Focus{Focus: false}}
		}
	}
	if k != q.focus {
		return nil
	}
	return q.events
}

func resolveFocus(op ui.Op, focus Key) (Key, listenerPriority, bool) {
	type childOp interface {
		ChildOp() ui.Op
	}
	var k Key
	var pri listenerPriority
	var hide bool
	switch op := op.(type) {
	case ui.Ops:
		for i := len(op) - 1; i >= 0; i-- {
			newK, newPri, h := resolveFocus(op[i], focus)
			hide = hide || h
			if newPri > pri {
				k, pri = newK, newPri
			}
		}
	case OpHandler:
		var newPri listenerPriority
		switch {
		case op.Focus:
			newPri = priNewFocus
		case op.Key == focus:
			newPri = priCurrentFocus
		default:
			newPri = priDefault
		}
		if newPri > pri {
			k, pri = op.Key, newPri
		}
	case OpHideInput:
		hide = true
	case childOp:
		return resolveFocus(op.ChildOp(), focus)
	}
	return k, pri, hide
}
