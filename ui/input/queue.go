// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"gioui.org/ui"
	"gioui.org/ui/key"
	"gioui.org/ui/pointer"
)

// Queue is an Events implementation that merges events from
// all available input sources.
type Queue struct {
	pqueue pointerQueue
	kqueue keyQueue

	handlers handlerEvents
}

type handlerEvents map[Key][]Event

func (q *Queue) Next(k Key) (Event, bool) {
	events := q.handlers[k]
	if len(events) == 0 {
		return nil, false
	}
	e := events[0]
	q.handlers[k] = events[1:]
	return e, true
}

func (q *Queue) Frame(ops *ui.Ops) {
	q.init()
	for k := range q.handlers {
		delete(q.handlers, k)
	}
	q.pqueue.Frame(ops, q.handlers)
	q.kqueue.Frame(ops, q.handlers)
}

func (q *Queue) Add(e Event) {
	q.init()
	switch e := e.(type) {
	case pointer.Event:
		q.pqueue.Push(e, q.handlers)
	case key.EditEvent, key.ChordEvent, key.FocusEvent:
		q.kqueue.Push(e, q.handlers)
	}
}

func (q *Queue) InputState() key.TextInputState {
	return q.kqueue.InputState()
}

func (q *Queue) init() {
	if q.handlers == nil {
		q.handlers = make(handlerEvents)
	}
}
