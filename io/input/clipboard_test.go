// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"io"
	"strings"
	"testing"

	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/transfer"
	"gioui.org/op"
)

func TestClipboardDuplicateEvent(t *testing.T) {
	ops, router, handler := new(op.Ops), new(Router), make([]int, 2)

	// Both must receive the event once
	router.Source().Execute(clipboard.ReadCmd{Tag: &handler[0]})
	router.Source().Execute(clipboard.ReadCmd{Tag: &handler[1]})

	router.Frame(ops)
	event := transfer.DataEvent{
		Type: "application/text",
		Open: func() io.ReadCloser {
			return io.NopCloser(strings.NewReader("Test"))
		},
	}
	router.Queue(event)
	assertClipboardEvent(t, router.Events(&handler[0], transfer.TargetFilter{Type: "application/text"}), true)
	assertClipboardEvent(t, router.Events(&handler[1], transfer.TargetFilter{Type: "application/text"}), true)
	assertClipboardReadCmd(t, router, 0)
	ops.Reset()

	// No ReadCmd

	router.Frame(ops)
	assertClipboardReadCmd(t, router, 0)
	assertClipboardEvent(t, router.Events(&handler[0]), false)
	assertClipboardEvent(t, router.Events(&handler[1]), false)
	ops.Reset()

	router.Source().Execute(clipboard.ReadCmd{Tag: &handler[0]})

	router.Frame(ops)
	// No ClipboardEvent sent
	assertClipboardReadCmd(t, router, 1)
	assertClipboardEvent(t, router.Events(&handler[0]), false)
	assertClipboardEvent(t, router.Events(&handler[1]), false)
	ops.Reset()
}

func TestQueueProcessReadClipboard(t *testing.T) {
	ops, router, handler := new(op.Ops), new(Router), make([]int, 2)
	ops.Reset()

	// Request read
	router.Source().Execute(clipboard.ReadCmd{Tag: &handler[0]})

	router.Frame(ops)
	assertClipboardReadCmd(t, router, 1)
	ops.Reset()

	for i := 0; i < 3; i++ {
		// No ReadCmd
		// One receiver must still wait for response

		router.Frame(ops)
		assertClipboardReadDuplicated(t, router, 1)
		ops.Reset()
	}

	router.Frame(ops)
	// Send the clipboard event
	event := transfer.DataEvent{
		Type: "application/text",
		Open: func() io.ReadCloser {
			return io.NopCloser(strings.NewReader("Text 2"))
		},
	}
	router.Queue(event)
	assertClipboardEvent(t, router.Events(&handler[0], transfer.TargetFilter{Type: "application/text"}), true)
	assertClipboardReadCmd(t, router, 0)
	ops.Reset()

	// No ReadCmd
	// There's no receiver waiting

	router.Frame(ops)
	assertClipboardReadCmd(t, router, 0)
	assertClipboardEvent(t, router.Events(&handler[0]), false)
	ops.Reset()
}

func TestQueueProcessWriteClipboard(t *testing.T) {
	router := new(Router)

	const mime = "application/text"
	router.Source().Execute(clipboard.WriteCmd{Type: mime, Data: io.NopCloser(strings.NewReader("Write 1"))})

	assertClipboardWriteCmd(t, router, mime, "Write 1")
	assertClipboardWriteCmd(t, router, "", "")

	router.Source().Execute(clipboard.WriteCmd{Type: mime, Data: io.NopCloser(strings.NewReader("Write 2"))})

	assertClipboardReadCmd(t, router, 0)
	assertClipboardWriteCmd(t, router, mime, "Write 2")
}

func assertClipboardEvent(t *testing.T, events []event.Event, expected bool) {
	t.Helper()
	var evtClipboard int
	for _, e := range events {
		switch e.(type) {
		case transfer.DataEvent:
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

func assertClipboardReadCmd(t *testing.T, router *Router, expected int) {
	t.Helper()
	if got := len(router.lastState().receivers); got != expected {
		t.Errorf("unexpected %d receivers, got %d", expected, got)
	}
	if router.ClipboardRequested() != (expected > 0) {
		t.Error("missing requests")
	}
}

func assertClipboardReadDuplicated(t *testing.T, router *Router, expected int) {
	t.Helper()
	if len(router.lastState().receivers) != expected {
		t.Error("receivers removed")
	}
	if router.ClipboardRequested() != false {
		t.Error("duplicated requests")
	}
}

func assertClipboardWriteCmd(t *testing.T, router *Router, mimeExp, expected string) {
	t.Helper()
	if (router.cqueue.text != nil) != (expected != "") {
		t.Error("text not defined")
	}
	mime, text, ok := router.cqueue.WriteClipboard()
	if ok != (expected != "") {
		t.Error("duplicated requests")
	}
	if string(mime) != mimeExp {
		t.Errorf("got MIME type %s, expected %s", mime, mimeExp)
	}
	if string(text) != expected {
		t.Errorf("got text %s, expected %s", text, expected)
	}
}
