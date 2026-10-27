// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos_test

package app

import "testing"

func TestOHOSWindowRouter(t *testing.T) {
	var r ohosWindowRouter[int]
	a := r.assign()
	b := r.assign()
	if a == 0 || b == 0 || a == b {
		t.Fatalf("ids = %d,%d", a, b)
	}
	if !r.stage(a, ohosEvent{kind: ohosEvTouch, x: 1}) {
		t.Fatal("stage failed")
	}
	pending := r.register(a, 10)
	if len(pending) != 1 || pending[0].x != 1 {
		t.Fatalf("pending = %#v", pending)
	}
	r.register(b, 20)
	if v, ok := r.get(0); !ok || v != 10 {
		t.Fatalf("default = %v,%v", v, ok)
	}
	if v, ok := r.get(b); !ok || v != 20 {
		t.Fatalf("get b = %v,%v", v, ok)
	}
	if got := r.all(); len(got) != 2 || got[0] != 10 || got[1] != 20 {
		t.Fatalf("all = %v", got)
	}
	r.unregister(a)
	if v, ok := r.get(0); !ok || v != 20 {
		t.Fatalf("default after unregister = %v,%v", v, ok)
	}
	r.unregister(b)
	if _, ok := r.get(0); ok {
		t.Fatal("router must be empty")
	}
	if r.stage(b, ohosEvent{kind: ohosEvTouch}) {
		t.Fatal("events for a destroyed window must be dropped")
	}
}
