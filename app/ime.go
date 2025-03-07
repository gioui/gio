// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"unicode"
	"unicode/utf16"
	"unicode/utf8"

	"gioui.org/io/input"
	"gioui.org/io/key"
)

type editorState struct {
	input.EditorState
	compose key.Range
}

func (e *editorState) Replace(r key.Range, text string) {
	if r.Start > r.End {
		r.Start, r.End = r.End, r.Start
	}
	runes := []rune(text)
	newEnd := r.Start + len(runes)
	adjust := func(pos int) int {
		switch {
		case newEnd < pos && pos <= r.End:
			return newEnd
		case r.End < pos:
			diff := newEnd - r.End
			return pos + diff
		}
		return pos
	}
	e.Selection.Start = adjust(e.Selection.Start)
	e.Selection.End = adjust(e.Selection.End)
	if e.compose.Start != -1 {
		e.compose.Start = adjust(e.compose.Start)
		e.compose.End = adjust(e.compose.End)
	}
	s := e.Snippet
	if r.End < s.Start || r.Start > s.End {
		// Discard snippet if it doesn't overlap with replacement.
		s = key.Snippet{
			Range: key.Range{
				Start: r.Start,
				End:   r.Start,
			},
		}
	}
	var newSnippet []rune
	snippet := []rune(s.Text)
	// Append first part of existing snippet.
	if end := r.Start - s.Start; end > 0 {
		newSnippet = append(newSnippet, snippet[:end]...)
	}
	// Append replacement.
	newSnippet = append(newSnippet, runes...)
	// Append last part of existing snippet.
	if start := r.End; start < s.End {
		newSnippet = append(newSnippet, snippet[start-s.Start:]...)
	}
	// Adjust snippet range to include replacement.
	if r.Start < s.Start {
		s.Start = r.Start
	}
	s.End = s.Start + len(newSnippet)
	s.Text = string(newSnippet)
	e.Snippet = s
}

// UTF16Index converts the given index in runes into an index in utf16 characters.
func (e *editorState) UTF16Index(runes int) int {
	if runes == -1 {
		return -1
	}
	if runes < e.Snippet.Start {
		// Assume runes before sippet are one UTF-16 character each.
		return runes
	}
	chars := e.Snippet.Start
	runes -= e.Snippet.Start
	for _, r := range e.Snippet.Text {
		if runes == 0 {
			break
		}
		runes--
		chars++
		if r1, _ := utf16.EncodeRune(r); r1 != unicode.ReplacementChar {
			chars++
		}
	}
	// Assume runes after snippets are one UTF-16 character each.
	return chars + runes
}

// RunesIndex converts the given index in utf16 characters to an index in runes.
func (e *editorState) RunesIndex(chars int) int {
	if chars == -1 {
		return -1
	}
	if chars < e.Snippet.Start {
		// Assume runes before offset are one UTF-16 character each.
		return chars
	}
	runes := e.Snippet.Start
	chars -= e.Snippet.Start
	for _, r := range e.Snippet.Text {
		if chars == 0 {
			break
		}
		chars--
		runes++
		if r1, _ := utf16.EncodeRune(r); r1 != unicode.ReplacementChar {
			chars--
		}
	}
	// Assume runes after snippets are one UTF-16 character each.
	return runes + chars
}

// areSnippetsConsistent reports whether the content of the old snippet is
// consistent with the content of the new.
func areSnippetsConsistent(old, new key.Snippet) bool {
	// Compute the overlapping range.
	r := old.Range
	r.Start = max(r.Start, new.Start)
	r.End = max(r.End, r.Start)
	r.End = min(r.End, new.End)
	return snippetSubstring(old, r) == snippetSubstring(new, r)
}

func snippetSubstring(s key.Snippet, r key.Range) string {
	for r.Start > s.Start && r.Start < s.End {
		_, n := utf8.DecodeRuneInString(s.Text)
		s.Text = s.Text[n:]
		s.Start++
	}
	for r.End < s.End && r.End > s.Start {
		_, n := utf8.DecodeLastRuneInString(s.Text)
		s.Text = s.Text[:len(s.Text)-n]
		s.End--
	}
	return s.Text
}
