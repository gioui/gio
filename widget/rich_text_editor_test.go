// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"testing"
)

func TestRichTextEditorHitTestUsesGlyphMidpoints(t *testing.T) {
	editor := NewRichTextEditor()
	editor.SetText("абв", CharStyle{S: 14})
	editor.SetLayoutFromExternal(
		[]image.Rectangle{
			image.Rect(0, 0, 10, 16),
			image.Rect(10, 0, 20, 16),
			image.Rect(20, 0, 30, 16),
		},
		[]rune{'а', 'б', 'в'},
	)

	checks := []struct {
		x    int
		want int
	}{
		{0, 0},
		{4, 0},
		{6, 1},
		{14, 1},
		{16, 2},
		{29, 3},
		{40, 3},
	}
	for _, tc := range checks {
		if !editor.SetCaretFromClick(tc.x, 8) {
			t.Fatalf("SetCaretFromClick(%d) returned false", tc.x)
		}
		if got := editor.CaretPos(); got != tc.want {
			t.Errorf("SetCaretFromClick(%d): got caret %d, want %d", tc.x, got, tc.want)
		}
	}
}

func TestRichTextEditorHitTestSelectsNearestWrappedLine(t *testing.T) {
	editor := NewRichTextEditor()
	editor.SetText("абвг", CharStyle{S: 14})
	editor.SetLayoutFromExternal(
		[]image.Rectangle{
			image.Rect(0, 0, 10, 16),
			image.Rect(10, 0, 20, 16),
			image.Rect(0, 20, 10, 36),
			image.Rect(10, 20, 20, 36),
		},
		[]rune{'а', 'б', 'в', 'г'},
	)

	if !editor.SetCaretFromClick(7, 28) {
		t.Fatal("SetCaretFromClick returned false")
	}
	if got, want := editor.CaretPos(), 3; got != want {
		t.Fatalf("wrapped-line click: got caret %d, want %d", got, want)
	}
	coords := editor.CaretCoords()
	if coords.X != 10 || coords.Y != 20 {
		t.Fatalf("caret coords = (%d,%d), want (10,20)", coords.X, coords.Y)
	}
}
