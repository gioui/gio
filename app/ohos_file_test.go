// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos_test

package app

import (
	"bytes"
	"io"
	"testing"

	"gioui.org/io/transfer"
)

func TestOHOSPumpDrop(t *testing.T) {
	p, f := newTestPump()
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 100, h: 100})
	p.Handle(ohosEvent{kind: ohosEvFrame})
	f.events = nil
	p.Handle(ohosEvent{kind: ohosEvDrop, text: `["/tmp/a b.ofd","/tmp/中文.pdf"]`})
	if len(f.events) != 1 {
		t.Fatalf("events = %#v", f.events)
	}
	de, ok := f.events[0].(FileDropEvent)
	if !ok || len(de.Paths) != 2 || de.Paths[0] != "/tmp/a b.ofd" || de.Paths[1] != "/tmp/中文.pdf" {
		t.Fatalf("drop = %#v", f.events[0])
	}
}

func TestOHOSPumpDropLegacyNewlines(t *testing.T) {
	p, f := newTestPump()
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 100, h: 100})
	p.Handle(ohosEvent{kind: ohosEvFrame})
	f.events = nil
	p.Handle(ohosEvent{kind: ohosEvDrop, text: "/a\n\n/b\n"})
	de, ok := f.events[0].(FileDropEvent)
	if !ok || len(de.Paths) != 2 || de.Paths[0] != "/a" || de.Paths[1] != "/b" {
		t.Fatalf("legacy drop = %#v", f.events[0])
	}
}

func TestOHOSPumpClipboardData(t *testing.T) {
	p, f := newTestPump()
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 100, h: 100})
	p.Handle(ohosEvent{kind: ohosEvFrame})
	f.events = nil
	want := []byte{'a', 0, 'b', 0xff}
	p.Handle(ohosEvent{kind: ohosEvClipboardData, text: "application/octet-stream", data: want})
	if len(f.events) != 1 {
		t.Fatalf("events = %#v", f.events)
	}
	de, ok := f.events[0].(transfer.DataEvent)
	if !ok || de.Type != "application/octet-stream" {
		t.Fatalf("data event = %#v", f.events[0])
	}
	rc := de.Open()
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("payload = %v", got)
	}
	// The event must own a copy of the queue's buffer.
	want[0] = 'z'
	rc = de.Open()
	got, _ = io.ReadAll(rc)
	rc.Close()
	if !bytes.Equal(got, []byte{'a', 0, 'b', 0xff}) {
		t.Fatalf("payload mutated: %v", got)
	}
}
