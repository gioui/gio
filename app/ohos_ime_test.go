// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos_test

package app

import (
	"testing"

	"gioui.org/io/key"
)

func imeState(text string, start, end, selStart, selEnd int) editorState {
	var st editorState
	st.Snippet = key.Snippet{Range: key.Range{Start: start, End: end}, Text: text}
	st.Selection.Range = key.Range{Start: selStart, End: selEnd}
	st.compose = key.Range{Start: -1, End: -1}
	return st
}

func TestOHOSDeleteRange(t *testing.T) {
	st := imeState("hello world", 0, 11, 5, 5)
	r, ok := ohosDeleteRange(st, 1, 0)
	if !ok || r != (key.Range{Start: 4, End: 5}) {
		t.Fatalf("backspace = %v,%v", r, ok)
	}
	r, _ = ohosDeleteRange(st, 0, 1)
	if r != (key.Range{Start: 5, End: 6}) {
		t.Fatalf("forward delete = %v", r)
	}
	r, _ = ohosDeleteRange(st, 100, 100)
	if r.Start != 0 || r.End != 11 {
		t.Fatalf("clamped = %v", r)
	}
	if _, ok := ohosDeleteRange(st, -1, 0); ok {
		t.Fatal("negative counts must fail")
	}
}

func TestOHOSMoveRange(t *testing.T) {
	st := imeState("abc", 0, 3, 1, 1)
	r, ok := ohosMoveRange(st, 3)
	if !ok || r != (key.Range{Start: 0, End: 0}) {
		t.Fatalf("left = %v", r)
	}
	r, _ = ohosMoveRange(st, 4)
	if r != (key.Range{Start: 2, End: 2}) {
		t.Fatalf("right = %v", r)
	}
	st.Selection.Range = key.Range{Start: 0, End: 0}
	r, _ = ohosMoveRange(st, 3)
	if r.Start != 0 {
		t.Fatalf("left clamp = %v", r)
	}
	st.Selection.Range = key.Range{Start: 3, End: 3}
	r, _ = ohosMoveRange(st, 4)
	if r.Start != 3 {
		t.Fatalf("right clamp = %v", r)
	}
	if _, ok := ohosMoveRange(st, 1); ok {
		t.Fatal("up/down unsupported")
	}
}

func TestOHOSTextAround(t *testing.T) {
	st := imeState("hello world", 0, 11, 5, 5)
	left, right := ohosTextAround(st, 3)
	if left != "llo" || right != " wo" {
		t.Fatalf("around = %q,%q", left, right)
	}
	st.Selection.Range = key.Range{Start: 0, End: 0}
	left, right = ohosTextAround(st, 5)
	if left != "" || right != "hello" {
		t.Fatalf("start = %q,%q", left, right)
	}
	st.Selection.Range = key.Range{Start: 11, End: 11}
	left, right = ohosTextAround(st, 100)
	if left != "hello world" || right != "" {
		t.Fatalf("end = %q,%q", left, right)
	}
}
