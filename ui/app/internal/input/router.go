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

type handlerEvents struct {
	handlers map[input.Key][]input.Event
	updated  bool
}

func (q *Router) Events(k input.Key) []input.Event {
	return q.handlers.For(k)
}

func (q *Router) Frame(ops *ui.Ops) {
	q.handlers.Clear()
	q.pqueue.Frame(ops, &q.handlers)
	q.kqueue.Frame(ops, &q.handlers)
}

func (q *Router) Add(e input.Event) bool {
	switch e := e.(type) {
	case pointer.Event:
		q.pqueue.Push(e, &q.handlers)
	case key.EditEvent, key.ChordEvent, key.FocusEvent:
		q.kqueue.Push(e, &q.handlers)
	}
	return q.handlers.Updated()
}

func (q *Router) InputState() key.TextInputState {
	return q.kqueue.InputState()
}

func (h *handlerEvents) init() {
	if h.handlers == nil {
		h.handlers = make(map[input.Key][]input.Event)
	}
}

func (h *handlerEvents) Set(k input.Key, evts []input.Event) {
	h.init()
	h.handlers[k] = evts
	h.updated = true
}

func (h *handlerEvents) Add(k input.Key, e input.Event) {
	h.init()
	h.handlers[k] = append(h.handlers[k], e)
	h.updated = true
}

func (h *handlerEvents) Updated() bool {
	u := h.updated
	h.updated = false
	return u
}

func (h *handlerEvents) For(k input.Key) []input.Event {
	events := h.handlers[k]
	delete(h.handlers, k)
	return events
}

func (h *handlerEvents) Clear() {
	for k := range h.handlers {
		delete(h.handlers, k)
	}
}
