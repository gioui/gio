// SPDX-License-Identifier: Unlicense OR MIT

package app

import "testing"

func TestWindowProcessFileDropEvent(t *testing.T) {
	var w Window
	if !w.processEvent(FileDropEvent{Paths: []string{"/tmp/a.pdf"}}) {
		t.Fatal("FileDropEvent must be handled as a window event")
	}
	if len(w.coalesced.drops) != 1 {
		t.Fatalf("drops = %#v", w.coalesced.drops)
	}
}

func TestWindowNextFileDropEvent(t *testing.T) {
	var w Window
	w.coalesced.drops = []FileDropEvent{
		{Paths: []string{"/tmp/a.pdf"}},
		{},
	}
	e, ok := w.nextEvent()
	if !ok {
		t.Fatal("drop event not delivered")
	}
	fd, ok := e.(FileDropEvent)
	if !ok || len(fd.Paths) != 1 || fd.Paths[0] != "/tmp/a.pdf" {
		t.Fatalf("first drop = %#v", e)
	}
	e, ok = w.nextEvent()
	if !ok || len(e.(FileDropEvent).Paths) != 0 {
		t.Fatalf("empty drop = %#v", e)
	}
	if _, ok := w.nextEvent(); ok {
		t.Fatal("drop queue must be empty")
	}
}
