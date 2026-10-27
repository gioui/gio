// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos_test

package app

import (
	"testing"

	"gioui.org/f32"
	"gioui.org/io/key"
)

func TestOHOSIMEWireBuild(t *testing.T) {
	st := editorState{}
	st.Snippet = key.Snippet{Range: key.Range{Start: 0, End: 8}, Text: "hello 世界"}
	st.Selection.Range = key.Range{Start: 6, End: 6}
	st.compose = key.Range{Start: -1, End: -1}
	st.Selection.Transform = f32.AffineId()

	sn, status := textInputSurrounding(st)
	if status != surroundingReady {
		t.Fatalf("status = %v", status)
	}
	var ime ohosIME
	ime.setHint(key.HintText)
	ime.keyboard = true
	wire, ok := ime.build(st, sn, true)
	if !ok {
		t.Fatal("build failed")
	}
	if wire.text != "hello 世界" || wire.length != len(wire.text) {
		t.Fatalf("text/length = %q/%d", wire.text, wire.length)
	}
	if wire.start != 6 || wire.end != 6 {
		t.Fatalf("selection = %d,%d", wire.start, wire.end)
	}
	if wire.hint != 1 || !wire.keyboard {
		t.Fatalf("hint/keyboard = %d/%v", wire.hint, wire.keyboard)
	}
	if !ime.changed(wire) {
		t.Fatal("first wire must be sent")
	}
	if ime.changed(wire) {
		t.Fatal("identical wire must be deduplicated")
	}
	ime.resetWire()
	if !ime.changed(wire) {
		t.Fatal("resetWire must force a resend")
	}
}

func TestOHOSIMEWireSelectionClamp(t *testing.T) {
	st := editorState{}
	st.Snippet = key.Snippet{Range: key.Range{Start: 10, End: 13}, Text: "abc"}
	st.Selection.Range = key.Range{Start: 12, End: 11}
	st.compose = key.Range{Start: -1, End: -1}
	st.Selection.Transform = f32.AffineId()
	sn, status := textInputSurrounding(st)
	if status != surroundingReady {
		t.Fatalf("status = %v", status)
	}
	var ime ohosIME
	wire, ok := ime.build(st, sn, true)
	if !ok {
		t.Fatal("build failed")
	}
	if wire.start != 1 || wire.end != 2 {
		t.Fatalf("reversed selection must be normalized, got %d,%d", wire.start, wire.end)
	}
}

func TestOHOSIMEDeleteRunes(t *testing.T) {
	sn := textInputSnapshot{
		text:   "a你好b",
		start:  0,
		cursor: len("a你"),
		anchor: len("a你"),
	}
	r, ok := sn.convertRuneRange(1, 1)
	if !ok {
		t.Fatal("convertRuneRange failed")
	}
	if r.Start != 1 || r.End != 3 {
		t.Fatalf("range = %v", r)
	}
	if _, ok := sn.convertRuneRange(100, 0); ok {
		t.Fatal("over-delete must fail")
	}
	// UTF-8 boundary check: cursor in the middle of a rune must fail.
	sn.cursor = 2
	if _, _, ok := sn.byteDeltaForRunes(1, 0); ok {
		t.Fatal("mid-rune cursor must fail")
	}
}
