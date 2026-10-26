// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony || gio_ohos_test

package app

import "gioui.org/io/key"

// ohosDeleteRange converts an input-method deletion into a rune range.
// before/after count characters relative to the current selection.
func ohosDeleteRange(st editorState, before, after int) (key.Range, bool) {
	if before < 0 || after < 0 {
		return key.Range{}, false
	}
	sel := normRange(st.Selection.Range)
	start := sel.Start - before
	end := sel.End + after
	if start < 0 {
		start = 0
	}
	if min, max, ok := ohosSnippetRange(st); ok {
		if start < min {
			start = min
		}
		if end > max {
			end = max
		}
	}
	if end < start {
		end = start
	}
	return key.Range{Start: start, End: end}, true
}

// ohosMoveRange implements the IME cursor-move directions.  Only left/right
// are meaningful for the text editors Gio exposes; up/down return false.
func ohosMoveRange(st editorState, direction int) (key.Range, bool) {
	sel := normRange(st.Selection.Range)
	var pos int
	switch direction {
	case 3: // left
		pos = sel.Start - 1
	case 4: // right
		pos = sel.End + 1
	default:
		return key.Range{}, false
	}
	if min, max, ok := ohosSnippetRange(st); ok {
		if pos < min {
			pos = min
		}
		if pos > max {
			pos = max
		}
	}
	if pos < 0 {
		pos = 0
	}
	return key.Range{Start: pos, End: pos}, true
}

// ohosTextAround returns up to n runes immediately left and right of the
// cursor, taken from the editor snippet.
func ohosTextAround(st editorState, n int) (string, string) {
	if n <= 0 {
		return "", ""
	}
	runes := []rune(st.Snippet.Text)
	cursor := st.Selection.Range.Start - st.Snippet.Start
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	left, right := cursor-n, cursor+n
	if left < 0 {
		left = 0
	}
	if right > len(runes) {
		right = len(runes)
	}
	return string(runes[left:cursor]), string(runes[cursor:right])
}

// ohosSnippetRange returns the snippet bounds when the snippet carries text.
func ohosSnippetRange(st editorState) (int, int, bool) {
	if st.Snippet.Text == "" || st.Snippet.End <= st.Snippet.Start {
		return 0, 0, false
	}
	return st.Snippet.Start, st.Snippet.End, true
}
