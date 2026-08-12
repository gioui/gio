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

func shouldCancelComposition(old, new editorState) bool {
	return old.Selection.Range != new.Selection.Range || !areSnippetsConsistent(old.Snippet, new.Snippet)
}

// imeRange is the range currently owned by the IME. While composing, both
// preedit updates and commits replace that range; otherwise they replace the
// editor selection.
func imeRange(state editorState) key.Range {
	rng := state.compose
	if rng.Start == -1 {
		rng = state.Selection.Range
	}
	return normRange(rng)
}

// normRange makes text replacement independent of the selection direction.
func normRange(r key.Range) key.Range {
	if r.Start > r.End {
		r.Start, r.End = r.End, r.Start
	}
	return r
}

func (e *editorState) Replace(r key.Range, text string) {
	r = normRange(r)
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

const maxSurroundingTextBytes = 4000

type textInputUpdate struct {
	preedit          string
	preeditSelection key.Range
	preeditSet       bool
	commit           string
	commitSet        bool
	deleteBefore     uint32
	deleteAfter      uint32
}

type textInputSnapshot struct {
	text           string
	start          int
	cursor, anchor int
}

type surroundingStatus uint8

const (
	surroundingReady surroundingStatus = iota
	// surroundingAwaitingSnippet reports that a larger snippet was
	// requested. It arrives in a later frame.
	surroundingAwaitingSnippet
	surroundingUnavailable
)

// textInputState is the platform-independent state of an input method
// session, driven by protocol events and applied to the editor.
type textInputState struct {
	pending   textInputUpdate
	sent      textInputSnapshot
	sentValid bool
	// serial counts the commits sent to the input method.
	serial uint32
	// dirty reports whether the editor changed since the last commit.
	dirty bool
	// external reports whether any of those changes originated externally.
	external bool
	// stale reports if the most recent done serial didn't match a commit,
	stale bool
}

// reset clears the session state, but NOT the commit serial.
func (s *textInputState) reset() {
	s.pending = textInputUpdate{}
	s.sent = textInputSnapshot{}
	s.sentValid = false
	s.dirty = false
	s.external = false
	s.stale = false
}

// apply applies the pending update to the editor and clears it,
// reporting whether the editor state changed.
func (s *textInputState) apply(e *callbacks) bool {
	beforeState := e.EditorState()
	update := s.pending
	s.pending = textInputUpdate{}
	if state := e.EditorState(); state.compose.Start != -1 {
		compose := normRange(state.compose)
		e.EditorReplace(compose, "")
		e.SetComposingRegion(key.Range{Start: -1, End: -1})
		e.SetEditorSelection(key.Range{Start: compose.Start, End: compose.Start})
	}
	if update.deleteBefore != 0 || update.deleteAfter != 0 {
		if rng, ok := s.sent.convertRange(update.deleteBefore, update.deleteAfter); ok && rng.Start != rng.End {
			e.EditorReplace(rng, "")
			e.SetEditorSelection(key.Range{Start: rng.Start, End: rng.Start})
		}
	}
	if update.commitSet {
		rng := imeRange(e.EditorState())
		e.EditorReplace(rng, update.commit)
		pos := rng.Start + utf8.RuneCountInString(update.commit)
		e.SetEditorSelection(key.Range{Start: pos, End: pos})
	}
	if update.preeditSet {
		rng := imeRange(e.EditorState())
		e.EditorReplace(rng, update.preedit)
		compose := key.Range{
			Start: rng.Start,
			End:   rng.Start + utf8.RuneCountInString(update.preedit),
		}
		e.SetComposingRegion(compose)
		selection := key.Range{Start: compose.End, End: compose.End}
		start, ok1 := byteOffsetToRune(update.preedit, update.preeditSelection.Start)
		end, ok2 := byteOffsetToRune(update.preedit, update.preeditSelection.End)
		if ok1 && ok2 {
			selection.Start = compose.Start + start
			selection.End = compose.Start + end
		}
		e.SetEditorSelection(selection)
	}
	return e.EditorState() != beforeState
}

// applyDone applies the pending update and reports whether the editor
// state changed and the serial matched. A mismatched done still applies,
// but marks the state stale.
func (s *textInputState) applyDone(e *callbacks, serial uint32) bool {
	s.dirty = s.apply(e) || s.dirty
	s.stale = serial != s.serial
	return !s.stale && s.dirty
}

func (s *textInputState) clearPreedit(e *callbacks) {
	s.pending = textInputUpdate{}
	if e == nil {
		return
	}
	if state := e.EditorState(); state.compose.Start != -1 {
		compose := normRange(state.compose)
		e.EditorReplace(compose, "")
		e.SetComposingRegion(key.Range{Start: -1, End: -1})
		e.SetEditorSelection(key.Range{Start: compose.Start, End: compose.Start})
	}
}

// byteOffsetToRune converts a UTF-8 byte offset to a rune offset.
func byteOffsetToRune(text string, offset int) (int, bool) {
	if offset < 0 || offset > len(text) || offset < len(text) && !utf8.RuneStart(text[offset]) {
		return 0, false
	}
	return utf8.RuneCountInString(text[:offset]), true
}

// runeOffsetToByte converts a rune offset to a UTF-8 byte offset.
func runeOffsetToByte(text string, offset int) (int, bool) {
	if offset < 0 {
		return 0, false
	}
	for idx := range text {
		if offset == 0 {
			return idx, true
		}
		offset--
	}
	if offset == 0 {
		return len(text), true
	}
	return 0, false
}

// textInputSurrounding extracts surrounding text from the editor
// snippet, with the composition removed and the text windowed around
// the selection on UTF-8 boundaries.
func textInputSurrounding(state editorState) (textInputSnapshot, surroundingStatus) {
	if state.compose.Start != -1 {
		compose := normRange(state.compose)
		if compose.Start < state.Snippet.Start || compose.End > state.Snippet.End {
			return textInputSnapshot{}, surroundingAwaitingSnippet
		}
		state.Replace(compose, "")
		state.compose = key.Range{Start: -1, End: -1}
	}
	sel := state.Selection.Range
	if sel.Start < state.Snippet.Start || sel.Start > state.Snippet.End ||
		sel.End < state.Snippet.Start || sel.End > state.Snippet.End {
		return textInputSnapshot{}, surroundingAwaitingSnippet
	}
	cursor, ok := runeOffsetToByte(state.Snippet.Text, sel.Start-state.Snippet.Start)
	if !ok {
		return textInputSnapshot{}, surroundingUnavailable
	}
	anchor, ok := runeOffsetToByte(state.Snippet.Text, sel.End-state.Snippet.Start)
	if !ok {
		return textInputSnapshot{}, surroundingUnavailable
	}
	start, end := min(cursor, anchor), max(cursor, anchor)
	if end-start > maxSurroundingTextBytes {
		// Cursor and anchor are byte offsets into the text we send, so a
		// larger selection can't be expressed. GTK gives up here too.
		return textInputSnapshot{}, surroundingUnavailable
	}
	for end-start < maxSurroundingTextBytes {
		grew := false
		if start > 0 {
			_, n := utf8.DecodeLastRuneInString(state.Snippet.Text[:start])
			if end-start+n <= maxSurroundingTextBytes {
				start -= n
				grew = true
			}
		}
		if end < len(state.Snippet.Text) {
			_, n := utf8.DecodeRuneInString(state.Snippet.Text[end:])
			if end-start+n <= maxSurroundingTextBytes {
				end += n
				grew = true
			}
		}
		if !grew {
			break
		}
	}
	startRunes := utf8.RuneCountInString(state.Snippet.Text[:start])
	return textInputSnapshot{
		text:   state.Snippet.Text[start:end],
		start:  state.Snippet.Start + startRunes,
		cursor: cursor - start,
		anchor: anchor - start,
	}, surroundingReady
}

// convertRange converts a surrounding-text deletion, in bytes,
// relative to the selection, into a rune range.
func (s textInputSnapshot) convertRange(before, after uint32) (key.Range, bool) {
	selectionStart := min(s.cursor, s.anchor)
	selectionEnd := max(s.cursor, s.anchor)
	beforeStart := selectionStart - int(before)
	afterEnd := selectionEnd + int(after)
	if beforeStart < 0 || afterEnd > len(s.text) {
		return key.Range{}, false
	}
	beforeRune, ok := byteOffsetToRune(s.text, beforeStart)
	if !ok {
		return key.Range{}, false
	}
	afterRune, ok := byteOffsetToRune(s.text, afterEnd)
	if !ok {
		return key.Range{}, false
	}
	return key.Range{Start: s.start + beforeRune, End: s.start + afterRune}, true
}

// requestSurroundingText asks the editor for a snippet covering the
// selection and composition, with context on both sides.
func requestSurroundingText(e *callbacks) {
	state := e.EditorState()
	sel := normRange(state.Selection.Range)
	if state.compose.Start != -1 {
		compose := normRange(state.compose)
		sel.Start = min(sel.Start, compose.Start)
		sel.End = max(sel.End, compose.End)
	}
	const contextRunes = maxSurroundingTextBytes / 2
	e.SetEditorSnippet(key.Range{
		Start: max(0, sel.Start-contextRunes),
		End:   sel.End + contextRunes,
	})
}

// prepareSurroundingText returns the surrounding text, requesting a larger
// snippet if the current one is too small. The snippet only arrives in a
// later frame, so callers must retry from the editor state change.
func prepareSurroundingText(e *callbacks) (textInputSnapshot, surroundingStatus) {
	snapshot, status := textInputSurrounding(e.EditorState())
	if status == surroundingAwaitingSnippet {
		requestSurroundingText(e)
	}
	return snapshot, status
}
