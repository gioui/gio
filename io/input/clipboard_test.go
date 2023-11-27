// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"io"
	"strings"
	"testing"

	"gioui.org/io/clipboard"
	"gioui.org/io/transfer"
	"gioui.org/op"
)

func TestClipboardDuplicateEvent(t *testing.T) {
	ops, r, handlers := new(op.Ops), new(Router), make([]int, 2)

	// Both must receive the event once.
	r.Source().Execute(clipboard.ReadCmd{Tag: &handlers[0]})
	r.Source().Execute(clipboard.ReadCmd{Tag: &handlers[1]})

	event := transfer.DataEvent{
		Type: "application/text",
		Open: func() io.ReadCloser {
			return io.NopCloser(strings.NewReader("Test"))
		},
	}
	r.Queue(event)
	for i := range handlers {
		f := transfer.TargetFilter{Target: &handlers[i], Type: "application/text"}
		assertEventTypeSequence(t, events(r, -1, f), transfer.DataEvent{})
	}
	assertClipboardReadCmd(t, r, 0)

	r.Source().Execute(clipboard.ReadCmd{Tag: &handlers[0]})

	r.Frame(ops)
	// No ClipboardEvent sent
	assertClipboardReadCmd(t, r, 1)
	for i := range handlers {
		f := transfer.TargetFilter{Target: &handlers[i]}
		assertEventTypeSequence(t, events(r, -1, f))
	}
}

func TestQueueProcessReadClipboard(t *testing.T) {
	ops, r, handler := new(op.Ops), new(Router), make([]int, 2)

	// Request read
	r.Source().Execute(clipboard.ReadCmd{Tag: &handler[0]})

	assertClipboardReadCmd(t, r, 1)
	ops.Reset()

	for i := 0; i < 3; i++ {
		// No ReadCmd
		// One receiver must still wait for response

		r.Frame(ops)
		assertClipboardReadDuplicated(t, r, 1)
	}

	// Send the clipboard event
	event := transfer.DataEvent{
		Type: "application/text",
		Open: func() io.ReadCloser {
			return io.NopCloser(strings.NewReader("Text 2"))
		},
	}
	r.Queue(event)
	assertEventTypeSequence(t, events(r, -1, transfer.TargetFilter{Target: &handler[0], Type: "application/text"}), transfer.DataEvent{})
	assertClipboardReadCmd(t, r, 0)
}

func TestQueueProcessWriteClipboard(t *testing.T) {
	r := new(Router)

	const mime = "application/text"
	r.Source().Execute(clipboard.WriteCmd{Type: mime, Data: io.NopCloser(strings.NewReader("Write 1"))})

	assertClipboardWriteCmd(t, r, mime, "Write 1")
	assertClipboardWriteCmd(t, r, "", "")

	r.Source().Execute(clipboard.WriteCmd{Type: mime, Data: io.NopCloser(strings.NewReader("Write 2"))})

	assertClipboardReadCmd(t, r, 0)
	assertClipboardWriteCmd(t, r, mime, "Write 2")
}

func assertClipboardReadCmd(t *testing.T, router *Router, expected int) {
	t.Helper()
	if got := len(router.state().receivers); got != expected {
		t.Errorf("unexpected %d receivers, got %d", expected, got)
	}
	if router.ClipboardRequested() != (expected > 0) {
		t.Error("missing requests")
	}
}

func assertClipboardReadDuplicated(t *testing.T, router *Router, expected int) {
	t.Helper()
	if len(router.state().receivers) != expected {
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
