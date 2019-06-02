// SPDX-License-Identifier: Unlicense OR MIT

package key

import (
	"gioui.org/ui"
	"gioui.org/ui/internal/ops"
)

type Queue struct {
	focus    Key
	events   []Event
	handlers map[Key]bool
	reader   ui.OpsReader
}

type listenerPriority uint8

const (
	priNone listenerPriority = iota
	priDefault
	priCurrentFocus
	priNewFocus
)

func (q *Queue) Frame(root *ui.Ops) TextInputState {
	q.events = q.events[:0]
	q.reader.Reset(root)
	f, pri, hide := resolveFocus(&q.reader, q.focus)
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

func resolveFocus(r *ui.OpsReader, focus Key) (Key, listenerPriority, bool) {
	var k Key
	var pri listenerPriority
	var hide bool
loop:
	for {
		encOp, ok := r.Decode()
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
			case op.Key == focus:
				newPri = priCurrentFocus
			default:
				newPri = priDefault
			}
			if newPri >= pri {
				k, pri = op.Key, newPri
			}
		case ops.TypeHideInput:
			hide = true
		case ops.TypePush:
			newK, newPri, h := resolveFocus(r, focus)
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
