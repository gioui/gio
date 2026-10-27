// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos_test

package app

import "testing"

func TestOHOSFlags(t *testing.T) {
	t.Setenv("GIO_OHOS_SYNC_FRAME", "")
	if ohosSyncFrameEnabled() {
		t.Fatal("sync frame must be off by default")
	}
	t.Setenv("GIO_OHOS_SYNC_FRAME", "1")
	if !ohosSyncFrameEnabled() {
		t.Fatal("sync frame flag not parsed")
	}
	t.Setenv("GIO_OHOS_NO_A11Y", "")
	if !ohosA11yEnabled() {
		t.Fatal("a11y must be on by default")
	}
	t.Setenv("GIO_OHOS_NO_A11Y", "true")
	if ohosA11yEnabled() {
		t.Fatal("a11y flag not parsed")
	}
}

func TestOHOSSyncFrameRendersImmediately(t *testing.T) {
	t.Setenv("GIO_OHOS_SYNC_FRAME", "1")
	p, f := newTestPump()
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 100, h: 100})
	if len(f.events) != 3 || f.frames != 0 {
		t.Fatalf("sync frame events=%#v frames=%d", f.events, f.frames)
	}
	if _, ok := f.events[2].(frameEvent); !ok {
		t.Fatalf("third event must be a frame: %#v", f.events[2])
	}
	p.Handle(ohosEvent{kind: ohosEvFrame})
	if len(f.events) != 3 {
		t.Fatalf("idle tick must not draw: %#v", f.events)
	}
}
