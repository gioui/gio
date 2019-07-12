// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"gioui.org/ui"
	"gioui.org/ui/input"
	"gioui.org/ui/key"
	"gioui.org/ui/pointer"
)

// Router is a Queue implementation that routes events from
// all available input sources to registered handlers.
type Router struct {
	pqueue pointerQueue
	kqueue keyQueue

	handlers handlerEvents
}

type handlerEvents map[input.Key][]input.Event

func (q *Router) Events(k input.Key) []input.Event {
	events := q.handlers[k]
	delete(q.handlers, k)
	return events
}

func (q *Router) Frame(ops *ui.Ops) {
	q.init()
	for k := range q.handlers {
		delete(q.handlers, k)
	}
	q.pqueue.Frame(ops, q.handlers)
	q.kqueue.Frame(ops, q.handlers)
}

func (q *Router) Add(e input.Event) {
	q.init()
	switch e := e.(type) {
	case pointer.Event:
		q.pqueue.Push(e, q.handlers)
	case key.EditEvent, key.ChordEvent, key.FocusEvent:
		q.kqueue.Push(e, q.handlers)
	}
}

func (q *Router) InputState() key.TextInputState {
	return q.kqueue.InputState()
}

func (q *Router) init() {
	if q.handlers == nil {
		q.handlers = make(handlerEvents)
	}
}
