package router

import (
	"gioui.org/io/transfer"
	"strings"
)

type transferQueue struct {
	initialized   bool
	schemes       []transfer.SchemeOp
	firstURLEvent transfer.URLEvent
}

func (q *transferQueue) Push(evt transfer.URLEvent, events *handlerEvents) {
	if !q.initialized && q.firstURLEvent.URL == nil {
		q.firstURLEvent = evt
		return
	}
	for _, op := range q.schemes {
		q.routeEvent(op, evt, events)
	}
}

func (q *transferQueue) ProcessSchemeOp(op transfer.SchemeOp, events *handlerEvents) {
	q.schemes = append(q.schemes, op)
	if !q.initialized && q.firstURLEvent.URL != nil {
		q.routeEvent(op, q.firstURLEvent, events)
	}
}

func (q *transferQueue) routeEvent(op transfer.SchemeOp, e transfer.URLEvent, events *handlerEvents) {
	if op.Scheme == "" || (e.URL != nil && strings.EqualFold(op.Scheme, e.URL.Scheme)) {
		events.Add(op.Tag, e)
	}
}

func (q *transferQueue) Clear() {
	if q.schemes != nil {
		q.schemes = q.schemes[:0]
	}
	if q.initialized {
		q.firstURLEvent = transfer.URLEvent{}
	}
	q.initialized = true
}
