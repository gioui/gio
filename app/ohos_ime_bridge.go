// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony || gio_ohos_test

package app

import (
	"image"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/io/key"
)

// ohosIMEWire is the last state sent to the platform. It is comparable so
// updateIME can avoid resending an unchanged snippet every frame.
type ohosIMEWire struct {
	text     string
	length   int
	start    int
	end      int
	hint     int
	caret    image.Rectangle
	keyboard bool
}

// ohosIME owns the editor-side IME session. It reuses textInputState so
// composition, replacement and selection semantics match the Wayland
// backend instead of maintaining a second state machine.
type ohosIME struct {
	state    textInputState
	sent     textInputSnapshot
	hasSent  bool
	last     ohosIMEWire
	hint     int
	keyboard bool
}

func (m *ohosIME) setHint(h key.InputHint) {
	m.hint = ohosInputHint(h)
}

// resetWire forces the next update to be sent even if the editor state
// happens to match the previous session.
func (m *ohosIME) resetWire() {
	m.last = ohosIMEWire{}
}

// build computes the wire state from a surrounding-text snapshot. It
// returns false when there is no usable snippet yet.
func (m *ohosIME) build(st editorState, sn textInputSnapshot, textInput bool) (ohosIMEWire, bool) {
	if !textInput || sn.cursor < 0 || sn.cursor > len(sn.text) || sn.anchor < 0 || sn.anchor > len(sn.text) {
		return ohosIMEWire{}, false
	}
	cursor := utf8.RuneCountInString(sn.text[:sn.cursor])
	anchor := utf8.RuneCountInString(sn.text[:sn.anchor])
	start, end := cursor, anchor
	if start > end {
		start, end = end, start
	}
	c := st.Selection
	top := c.Transform.Transform(c.Caret.Pos.Sub(f32.Pt(0, c.Caret.Ascent)))
	bottom := c.Transform.Transform(c.Caret.Pos.Add(f32.Pt(0, c.Caret.Descent)))
	x0, x1 := top.X, bottom.X
	if x0 > x1 {
		x0, x1 = x1, x0
	}
	y0, y1 := top.Y, bottom.Y
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	wire := ohosIMEWire{
		text:     sn.text,
		length:   len(sn.text),
		start:    start,
		end:      end,
		hint:     m.hint,
		caret:    image.Rect(int(x0), int(y0), int(x1), int(y1)),
		keyboard: m.keyboard,
	}
	m.sent = sn
	m.hasSent = true
	m.state.sent = sn
	return wire, true
}

// changed records wire and reports whether it differs from the last send.
func (m *ohosIME) changed(wire ohosIMEWire) bool {
	if wire == m.last {
		return false
	}
	m.last = wire
	return true
}

func (m *ohosIME) preedit(w *callbacks, text string, selStart, selEnd int) bool {
	var sel key.Range
	if selStart >= 0 && selEnd >= 0 {
		if b0, ok := runeOffsetToByte(text, selStart); ok {
			if b1, ok := runeOffsetToByte(text, selEnd); ok {
				sel = key.Range{Start: b0, End: b1}
			}
		}
	}
	m.state.pending = textInputUpdate{
		preedit:          text,
		preeditSelection: sel,
		preeditSet:       true,
	}
	return m.state.apply(w)
}

func (m *ohosIME) commit(w *callbacks, text string) bool {
	m.state.pending = textInputUpdate{commit: text, commitSet: true}
	return m.state.apply(w)
}

func (m *ohosIME) delete(w *callbacks, before, after int) bool {
	if !m.hasSent {
		return false
	}
	b, a, ok := m.sent.byteDeltaForRunes(before, after)
	if !ok {
		return false
	}
	m.state.sent = m.sent
	m.state.pending = textInputUpdate{deleteBefore: b, deleteAfter: a}
	return m.state.apply(w)
}

// selectRange applies an absolute document rune range, clamped to the
// current snippet. The platform uses absolute offsets for ime_set_selection.
func (m *ohosIME) selectRange(w *callbacks, start, end int) {
	st := w.EditorState()
	if sn := st.Snippet; sn.End > sn.Start {
		start = max(start, sn.Start)
		end = max(end, sn.Start)
		start = min(start, sn.End)
		end = min(end, sn.End)
	}
	if start > end {
		start, end = end, start
	}
	w.SetEditorSelection(key.Range{Start: start, End: end})
}
