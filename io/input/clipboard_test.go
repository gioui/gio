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

	for range 3 {
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

func TestClipboardWriteSensitive(t *testing.T) {
	r := new(Router)
	write := func(text string, sensitive bool) {
		r.Source().Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(text)), Sensitive: sensitive})
		r.Frame(new(op.Ops))
		if _, _, ok := r.WriteClipboard(); !ok {
			t.Fatalf("no write of %q", text)
		}
	}
	write("secret", true)
	if !r.ClipboardSensitive() {
		t.Error("a sensitive write is not marked as sensitive")
	}
	write("plain", false)
	if r.ClipboardSensitive() {
		t.Error("a plain write is marked as sensitive")
	}
}

func TestClipboardReadTypes(t *testing.T) {
	var r Router
	var text, image int
	r.Source().Execute(clipboard.ReadCmd{Tag: &text})
	r.Source().Execute(clipboard.ReadCmd{Tag: &image, Type: "image/png"})
	r.Source().Execute(clipboard.ReadCmd{Tag: &image, Type: "image/png"})
	if types := r.ClipboardTypes(); len(types) != 2 || types[0] != "application/text" || types[1] != "image/png" {
		t.Fatalf("unexpected requested types: %v", types)
	}
	open := func() io.ReadCloser { return io.NopCloser(strings.NewReader("")) }
	r.Queue(transfer.DataEvent{Type: "image/png", Open: open})
	assertEventTypeSequence(t, events(&r, -1, transfer.TargetFilter{Target: &image, Type: "image/png"}), transfer.DataEvent{})
	assertEventTypeSequence(t, events(&r, -1, transfer.TargetFilter{Target: &text, Type: "application/text"}))
	if types := r.ClipboardTypes(); len(types) != 1 || types[0] != "application/text" {
		t.Fatalf("text request lost: %v", types)
	}
	r.Queue(transfer.DataEvent{Type: "application/text", Open: open})
	assertEventTypeSequence(t, events(&r, -1, transfer.TargetFilter{Target: &text, Type: "application/text"}), transfer.DataEvent{})
	if types := r.ClipboardTypes(); len(types) != 0 {
		t.Fatalf("completed requests remain: %v", types)
	}
}

func TestClipboardWriteContents(t *testing.T) {
	var r Router
	r.Source().Execute(clipboard.WriteContentsCmd{Contents: []clipboard.Content{
		{Type: "application/text", Data: []byte("plain")},
		{Type: "text/html", Data: []byte("<b>plain</b>")},
	}})
	r.Frame(new(op.Ops))
	contents, ok := r.WriteClipboardContents()
	if !ok || len(contents) != 2 {
		t.Fatalf("got %v, %v", contents, ok)
	}
	if string(contents[0].Data) != "plain" || string(contents[1].Data) != "<b>plain</b>" {
		t.Fatalf("unexpected contents: %q, %q", contents[0].Data, contents[1].Data)
	}
}

func TestClipboardReadContents(t *testing.T) {
	var r Router
	var target int
	r.Source().Execute(clipboard.ReadContentsCmd{Tag: &target, Types: []string{"application/text", "text/html"}})
	if types := r.ClipboardTypes(); len(types) != 2 {
		t.Fatalf("unexpected types: %v", types)
	}
	r.Queue(transfer.DataEvent{Type: "application/text", Open: func() io.ReadCloser {
		return io.NopCloser(strings.NewReader("plain"))
	}})
	assertEventTypeSequence(t, events(&r, -1, transfer.TargetFilter{Target: &target, Type: "application/text"}), transfer.DataEvent{})
	r.Queue(transfer.DataEvent{Type: "text/html", Open: func() io.ReadCloser {
		return io.NopCloser(strings.NewReader("<b>plain</b>"))
	}})
	assertEventTypeSequence(t, events(&r, -1, transfer.TargetFilter{Target: &target, Type: "text/html"}), transfer.DataEvent{})
	if types := r.ClipboardTypes(); len(types) != 0 {
		t.Fatalf("completed requests remain: %v", types)
	}
}

func TestClipboardContentsCopyAndClear(t *testing.T) {
	var r Router
	data := []byte("plain")
	r.Source().Execute(clipboard.WriteContentsCmd{
		Contents:  []clipboard.Content{{Type: "application/text", Data: data}},
		Sensitive: true,
	})
	data[0] = 'X'
	contents, ok := r.WriteClipboardContents()
	if !ok || string(contents[0].Data) != "plain" || !r.ClipboardSensitive() {
		t.Fatalf("write did not retain its data and sensitivity: %v", contents)
	}
	r.Source().Execute(clipboard.WriteContentsCmd{})
	contents, ok = r.WriteClipboardContents()
	if !ok || len(contents) != 0 || r.ClipboardSensitive() {
		t.Fatal("empty write did not clear the clipboard")
	}
	if _, ok := r.WriteClipboardContents(); ok {
		t.Fatal("clipboard clear repeated")
	}
}
