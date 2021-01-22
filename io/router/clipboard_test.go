package router

import (
	"testing"

	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/op"
)

func TestClipboardDuplicateEvent(t *testing.T) {
	ops, router, handler := new(op.Ops), new(Router), make([]int, 2)

	// Both must receive the event once
	clipboard.ReadOp{Tag: &handler[0]}.Add(ops)
	clipboard.ReadOp{Tag: &handler[1]}.Add(ops)

	router.Frame(ops)
	event := clipboard.Event{Text: "Test"}
	router.Queue(event)
	assertClipboardReadOp(t, router, 0)
	assertClipboardEvent(t, router.Events(&handler[0]), true)
	assertClipboardEvent(t, router.Events(&handler[1]), true)
	ops.Reset()

	// No ReadOp

	router.Frame(ops)
	assertClipboardReadOp(t, router, 0)
	assertClipboardEvent(t, router.Events(&handler[0]), false)
	assertClipboardEvent(t, router.Events(&handler[1]), false)
	ops.Reset()

	clipboard.ReadOp{Tag: &handler[0]}.Add(ops)

	router.Frame(ops)
	// No ClipboardEvent sent
	assertClipboardReadOp(t, router, 1)
	assertClipboardEvent(t, router.Events(&handler[0]), false)
	assertClipboardEvent(t, router.Events(&handler[1]), false)
	ops.Reset()
}

func TestQueueProcessReadClipboard(t *testing.T) {
	ops, router, handler := new(op.Ops), new(Router), make([]int, 2)
	ops.Reset()

	// Request read
	clipboard.ReadOp{Tag: &handler[0]}.Add(ops)

	router.Frame(ops)
	assertClipboardReadOp(t, router, 1)
	ops.Reset()

	for i := 0; i < 3; i++ {
		// No ReadOp
		// One receiver must still wait for response

		router.Frame(ops)
		assertClipboardReadOpDuplicated(t, router, 1)
		ops.Reset()
	}

	router.Frame(ops)
	// Send the clipboard event
	event := clipboard.Event{Text: "Text 2"}
	router.Queue(event)
	assertClipboardReadOp(t, router, 0)
	assertClipboardEvent(t, router.Events(&handler[0]), true)
	ops.Reset()

	// No ReadOp
	// There's no receiver waiting

	router.Frame(ops)
	assertClipboardReadOp(t, router, 0)
	assertClipboardEvent(t, router.Events(&handler[0]), false)
	ops.Reset()
}

func TestQueueProcessWriteClipboard(t *testing.T) {
	ops, router := new(op.Ops), new(Router)
	ops.Reset()

	clipboard.WriteOp{Text: "Write 1"}.Add(ops)

	router.Frame(ops)
	assertClipboardWriteOp(t, router, "Write 1")
	ops.Reset()

	// No WriteOp

	router.Frame(ops)
	assertClipboardWriteOp(t, router, "")
	ops.Reset()

	clipboard.WriteOp{Text: "Write 2"}.Add(ops)

	router.Frame(ops)
	assertClipboardReadOp(t, router, 0)
	assertClipboardWriteOp(t, router, "Write 2")
	ops.Reset()
}

func assertClipboardEvent(t *testing.T, events []event.Event, expected bool) {
	t.Helper()
	var evtClipboard int
	for _, e := range events {
		switch e.(type) {
		case clipboard.Event:
			evtClipboard++
		}
	}
	if evtClipboard <= 0 && expected {
		t.Error("expected to receive some event")
	}
	if evtClipboard > 0 && !expected {
		t.Error("unexpected event received")
	}
}

func assertClipboardReadOp(t *testing.T, router *Router, expected int) {
	t.Helper()
	if len(router.cqueue.receivers) != expected {
		t.Error("unexpected number of receivers")
	}
	if router.cqueue.ReadClipboard() != (expected > 0) {
		t.Error("missing requests")
	}
}

func assertClipboardReadOpDuplicated(t *testing.T, router *Router, expected int) {
	t.Helper()
	if len(router.cqueue.receivers) != expected {
		t.Error("receivers removed")
	}
	if router.cqueue.ReadClipboard() != false {
		t.Error("duplicated requests")
	}
}

func assertClipboardWriteOp(t *testing.T, router *Router, expected string) {
	t.Helper()
	if (router.cqueue.text != nil) != (expected != "") {
		t.Error("text not defined")
	}
	text, ok := router.cqueue.WriteClipboard()
	if ok != (expected != "") {
		t.Error("duplicated requests")
	}
	if text != expected {
		t.Errorf("got text %s, expected %s", text, expected)
	}
}
