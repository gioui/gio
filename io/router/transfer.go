package router

import (
	"gioui.org/io/transfer"
	"strings"
)

type transferQueue struct {
	handlers      []transfer.SchemeOp
	firstURLEvent transfer.URLEvent
}

func (q *transferQueue) Push(evt transfer.URLEvent, events *handlerEvents) {
	if q.handlers == nil && q.firstURLEvent.URL == nil {
		q.firstURLEvent = evt
		return
	}
	for _, op := range q.handlers {
		q.routeEvent(op, evt, events)
	}
}

func (q *transferQueue) ProcessSchemeOp(op transfer.SchemeOp, events *handlerEvents) {
	q.handlers = append(q.handlers, op)
	if q.firstURLEvent.URL != nil {
		q.routeEvent(op, q.firstURLEvent, events)
	}
}

func (q *transferQueue) routeEvent(op transfer.SchemeOp, e transfer.URLEvent, events *handlerEvents) {
	if op.Scheme == "" || (e.URL != nil && strings.EqualFold(op.Scheme, e.URL.Scheme)) {
		events.Add(op.Tag, e)
	}
}

func (q *transferQueue) Clear() {
	if q.handlers != nil {
		q.handlers = q.handlers[:0]
		q.firstURLEvent = transfer.URLEvent{}
	} else {
		q.handlers = []transfer.SchemeOp{}
	}
}
