// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"io"
	"unicode"
	"unicode/utf8"

	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
)

// KeyInput is a minimal widget that handles keyboard input only.
// It doesn't render anything or handle pointer events.
type KeyInput struct {
	text           string
	caretPos       int
	selectionStart int
	selectionEnd   int
	focusTag       event.Tag
	pending        []KeyInputEvent
	hasFocus       bool

	// Rich text support: caret-associated style.
	// CaretStyle holds the current style (font, size, color, bold, etc.)
	// that will be applied to newly inserted characters.
	// The host application (e.g., WASM editor) sets this when the caret moves.
	CaretStyle interface{}

	// OnCaretMove is called whenever the caret position changes (arrow keys,
	// click, insert, delete). The callback receives the new caret position.
	// Use this to update CaretStyle based on the character to the left.
	OnCaretMove func(pos int)

	// OnSubmit is called when Enter or Tab is pressed.
	// The callback receives the current text content.
	OnSubmit func(text string)
}

// KeyInputEvent is the interface for events emitted by KeyInput.
type KeyInputEvent interface {
	isKeyInputEvent()
}

// TextInputEvent is generated when text changes.
type TextInputEvent struct {
	Text string
}

func (TextInputEvent) isKeyInputEvent() {}

// CursorMoveEvent is generated when the cursor moves.
type CursorMoveEvent struct {
	Position int
}

func (CursorMoveEvent) isKeyInputEvent() {}

// NewKeyInput creates a new KeyInput widget.
func NewKeyInput() *KeyInput {
	return &KeyInput{
		focusTag: new(event.Tag),
	}
}

// Text returns the current text content.
func (ki *KeyInput) Text() string {
	return ki.text
}

// SetText replaces the entire text content.
func (ki *KeyInput) SetText(text string) {
	ki.text = text
	runes := []rune(text)
	if ki.caretPos > len(runes) {
		ki.caretPos = len(runes)
	}
	ki.selectionStart = 0
	ki.selectionEnd = 0
}

// CaretPos returns the current caret position (rune index).
func (ki *KeyInput) CaretPos() int {
	return ki.caretPos
}

// SetCaret sets the caret position and optionally selection range.
func (ki *KeyInput) SetCaret(start, end int) {
	runes := []rune(ki.text)
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}
	if start > len(runes) {
		start = len(runes)
	}
	if end > len(runes) {
		end = len(runes)
	}
	ki.caretPos = start
	ki.selectionStart = start
	ki.selectionEnd = end
}

// moveCaretTo sets the caret position and fires OnCaretMove.
func (ki *KeyInput) moveCaretTo(pos int) {
	ki.caretPos = pos
	if ki.OnCaretMove != nil {
		ki.OnCaretMove(pos)
	}
}

// Selection returns the selection range (from, to) with from <= to.
func (ki *KeyInput) Selection() (int, int) {
	if ki.selectionStart <= ki.selectionEnd {
		return ki.selectionStart, ki.selectionEnd
	}
	return ki.selectionEnd, ki.selectionStart
}

// HasSelection returns true if there's an active selection.
func (ki *KeyInput) HasSelection() bool {
	return ki.selectionStart != ki.selectionEnd
}

// SelectedText returns the selected text.
func (ki *KeyInput) SelectedText() string {
	from, to := ki.Selection()
	runes := []rune(ki.text)
	if from >= len(runes) || to > len(runes) || from >= to {
		return ""
	}
	return string(runes[from:to])
}

// deleteSelection removes the selected text.
func (ki *KeyInput) deleteSelection() bool {
	from, to := ki.Selection()
	if from == to {
		return false
	}
	runes := []rune(ki.text)
	if from >= len(runes) || to > len(runes) {
		return false
	}
	ki.text = string(runes[:from]) + string(runes[to:])
	ki.caretPos = from
	ki.selectionStart = 0
	ki.selectionEnd = 0
	return true
}

// insertText inserts text at the caret position.
func (ki *KeyInput) insertText(text string) {
	if ki.HasSelection() {
		ki.deleteSelection()
	}
	runes := []rune(ki.text)
	if ki.caretPos > len(runes) {
		ki.caretPos = len(runes)
	}
	newRunes := make([]rune, 0, len(runes)+len([]rune(text)))
	newRunes = append(newRunes, runes[:ki.caretPos]...)
	newRunes = append(newRunes, []rune(text)...)
	newRunes = append(newRunes, runes[ki.caretPos:]...)
	ki.text = string(newRunes)
	ki.caretPos += len([]rune(text))
}

// Update processes events and returns any input events.
func (ki *KeyInput) Update(gtx layout.Context) (KeyInputEvent, bool) {
	if len(ki.pending) > 0 {
		out := ki.pending[0]
		ki.pending = ki.pending[1:]
		return out, true
	}

	filters := []event.Filter{
		key.FocusFilter{Target: ki.focusTag},
		transfer.TargetFilter{Target: ki.focusTag, Type: "application/text"},
		key.Filter{Focus: ki.focusTag, Name: key.NameLeftArrow, Optional: key.ModShortcutAlt | key.ModShift},
		key.Filter{Focus: ki.focusTag, Name: key.NameRightArrow, Optional: key.ModShortcutAlt | key.ModShift},
		key.Filter{Focus: ki.focusTag, Name: key.NameUpArrow, Optional: key.ModShift},
		key.Filter{Focus: ki.focusTag, Name: key.NameDownArrow, Optional: key.ModShift},
		key.Filter{Focus: ki.focusTag, Name: key.NameHome, Optional: key.ModShift},
		key.Filter{Focus: ki.focusTag, Name: key.NameEnd, Optional: key.ModShift},
		key.Filter{Focus: ki.focusTag, Name: key.NameDeleteBackward, Optional: key.ModShortcutAlt | key.ModShift},
		key.Filter{Focus: ki.focusTag, Name: key.NameDeleteForward, Optional: key.ModShortcutAlt | key.ModShift},
		key.Filter{Focus: ki.focusTag, Name: key.NameReturn, Optional: key.ModShift},
		key.Filter{Focus: ki.focusTag, Name: key.NameEnter, Optional: key.ModShift},
		key.Filter{Focus: ki.focusTag, Name: key.NameSpace, Optional: key.ModShift},
		key.Filter{Focus: ki.focusTag, Name: "A", Required: key.ModShortcut},
		key.Filter{Focus: ki.focusTag, Name: "C", Required: key.ModShortcut},
		key.Filter{Focus: ki.focusTag, Name: "V", Required: key.ModShortcut},
		key.Filter{Focus: ki.focusTag, Name: "X", Required: key.ModShortcut},
		key.Filter{Focus: ki.focusTag, Name: "Z", Required: key.ModShortcut, Optional: key.ModShift},
		// Catch-all: match any key event not matched above (regular characters).
		// Optional modifiers: Shift, Alt, AltGr (Ctrl+Alt on some keyboards).
		key.Filter{Focus: ki.focusTag, Optional: key.ModShift | key.ModAlt},
	}

	for {
		evt, ok := gtx.Event(filters...)
		if !ok {
			break
		}

		switch e := evt.(type) {
		case key.FocusEvent:
			if e.Focus {
				gtx.Execute(key.SoftKeyboardCmd{Show: true})
			}
			ki.hasFocus = e.Focus

		case transfer.DataEvent:
			// Clipboard paste
			if e.Open != nil {
				reader := e.Open()
				defer reader.Close()
				buf := make([]byte, 1024*1024)
				n, _ := reader.Read(buf)
				if n > 0 {
					text := string(buf[:n])
					ki.insertText(text)
					return TextInputEvent{Text: ki.Text()}, true
				}
			}

		case key.Event:
			if e.State == key.Press {
				ev := ki.processKeyEvent(gtx, e)
				if ev != nil {
					return ev, true
				}
			}
		}
	}

	return nil, false
}

// processKeyEvent handles a single key event.
func (ki *KeyInput) processKeyEvent(gtx layout.Context, e key.Event) KeyInputEvent {
	hasShift := e.Modifiers&key.ModShift != 0
	hasShortcut := e.Modifiers&key.ModShortcut != 0

	if hasShortcut {
		return ki.processShortcut(gtx, e)
	}

	switch e.Name {
	case key.NameLeftArrow:
		if hasShift {
			if ki.selectionStart == ki.selectionEnd {
				ki.selectionStart = ki.caretPos
				ki.selectionEnd = ki.caretPos
			}
			if ki.caretPos > 0 {
				ki.caretPos--
			}
			ki.selectionEnd = ki.caretPos
		} else {
			if ki.HasSelection() {
				from, _ := ki.Selection()
				ki.caretPos = from
			} else if ki.caretPos > 0 {
				ki.caretPos--
			}
			ki.selectionStart = 0
			ki.selectionEnd = 0
		}
		if ki.OnCaretMove != nil {
			ki.OnCaretMove(ki.caretPos)
		}
		return CursorMoveEvent{Position: ki.caretPos}

	case key.NameRightArrow:
		if hasShift {
			if ki.selectionStart == ki.selectionEnd {
				ki.selectionStart = ki.caretPos
				ki.selectionEnd = ki.caretPos
			}
			runes := []rune(ki.text)
			if ki.caretPos < len(runes) {
				ki.caretPos++
			}
			ki.selectionEnd = ki.caretPos
		} else {
			if ki.HasSelection() {
				_, to := ki.Selection()
				ki.caretPos = to
			} else {
				runes := []rune(ki.text)
				if ki.caretPos < len(runes) {
					ki.caretPos++
				}
			}
			ki.selectionStart = 0
			ki.selectionEnd = 0
		}
		if ki.OnCaretMove != nil {
			ki.OnCaretMove(ki.caretPos)
		}
		return CursorMoveEvent{Position: ki.caretPos}

	case key.NameUpArrow:
		ki.caretPos = 0
		ki.selectionStart = 0
		ki.selectionEnd = 0
		if ki.OnCaretMove != nil {
			ki.OnCaretMove(ki.caretPos)
		}
		return CursorMoveEvent{Position: ki.caretPos}

	case key.NameDownArrow:
		runes := []rune(ki.text)
		ki.caretPos = len(runes)
		ki.selectionStart = 0
		ki.selectionEnd = 0
		if ki.OnCaretMove != nil {
			ki.OnCaretMove(ki.caretPos)
		}
		return CursorMoveEvent{Position: ki.caretPos}

	case key.NameHome:
		ki.caretPos = 0
		ki.selectionStart = 0
		ki.selectionEnd = 0
		if ki.OnCaretMove != nil {
			ki.OnCaretMove(ki.caretPos)
		}
		return CursorMoveEvent{Position: ki.caretPos}

	case key.NameEnd:
		runes := []rune(ki.text)
		ki.caretPos = len(runes)
		ki.selectionStart = 0
		ki.selectionEnd = 0
		if ki.OnCaretMove != nil {
			ki.OnCaretMove(ki.caretPos)
		}
		return CursorMoveEvent{Position: ki.caretPos}

	case key.NameDeleteBackward:
		if ki.HasSelection() {
			ki.deleteSelection()
		} else if ki.caretPos > 0 {
			runes := []rune(ki.text)
			ki.text = string(runes[:ki.caretPos-1]) + string(runes[ki.caretPos:])
			ki.caretPos--
		}
		if ki.OnCaretMove != nil {
			ki.OnCaretMove(ki.caretPos)
		}
		return TextInputEvent{Text: ki.Text()}

	case key.NameDeleteForward:
		if ki.HasSelection() {
			ki.deleteSelection()
		} else {
			runes := []rune(ki.text)
			if ki.caretPos < len(runes) {
				ki.text = string(runes[:ki.caretPos]) + string(runes[ki.caretPos+1:])
			}
		}
		if ki.OnCaretMove != nil {
			ki.OnCaretMove(ki.caretPos)
		}
		return TextInputEvent{Text: ki.Text()}

	case key.NameReturn, key.NameEnter:
		if ki.OnSubmit != nil {
			ki.OnSubmit(ki.text)
		}
		ki.insertText("\n")
		return TextInputEvent{Text: ki.Text()}

	case key.NameSpace:
		ki.insertText(" ")
		return TextInputEvent{Text: ki.Text()}
	}

	// Regular character input — accept ALL printable Unicode characters
	// (Cyrillic, Latin extended, CJK, etc.)
	// Force case based on Shift state so the result is correct regardless
	// of what translateKey sends (WASM may uppercase or pass through as-is).
	if len(e.Name) == 1 {
		r := rune(e.Name[0])
		if unicode.IsPrint(r) {
			if hasShift {
				r = unicode.ToUpper(r)
			} else {
				r = unicode.ToLower(r)
			}
			ki.insertText(string(r))
			return TextInputEvent{Text: ki.Text()}
		}
	}

	// Multi-byte characters (Cyrillic, CJK, etc.) — single runes only.
	if len(e.Name) > 1 && !hasShortcut {
		r, s := utf8.DecodeRuneInString(string(e.Name))
		if s == len(string(e.Name)) && unicode.IsPrint(r) {
			if hasShift {
				r = unicode.ToUpper(r)
			} else {
				r = unicode.ToLower(r)
			}
			ki.insertText(string(r))
			return TextInputEvent{Text: ki.Text()}
		}
	}

	return nil
}

// processShortcut handles Ctrl/Cmd shortcuts.
func (ki *KeyInput) processShortcut(gtx layout.Context, e key.Event) KeyInputEvent {
	switch e.Name {
	case "A":
		ki.SetCaret(0, len([]rune(ki.text)))
		ki.selectionEnd = len([]rune(ki.text))
		if ki.OnCaretMove != nil {
			ki.OnCaretMove(ki.caretPos)
		}
		return CursorMoveEvent{Position: ki.caretPos}

	case "C":
		if ki.HasSelection() {
			text := ki.SelectedText()
			if text != "" {
				gtx.Execute(clipboard.WriteCmd{
					Type: "application/text",
					Data: &readCloser{data: text},
				})
			}
		}

	case "X":
		if ki.HasSelection() {
			text := ki.SelectedText()
			if text != "" {
				gtx.Execute(clipboard.WriteCmd{
					Type: "application/text",
					Data: &readCloser{data: text},
				})
			}
			ki.deleteSelection()
			if ki.OnCaretMove != nil {
				ki.OnCaretMove(ki.caretPos)
			}
			return TextInputEvent{Text: ki.Text()}
		}

	case "V":
		gtx.Execute(clipboard.ReadCmd{
			Tag: ki.focusTag,
		})

	case "Z":
		// TODO: undo

	case "Home":
		ki.SetCaret(0, 0)
		if ki.OnCaretMove != nil {
			ki.OnCaretMove(ki.caretPos)
		}
		return CursorMoveEvent{Position: ki.caretPos}

	case "End":
		runes := []rune(ki.text)
		ki.SetCaret(len(runes), len(runes))
		if ki.OnCaretMove != nil {
			ki.OnCaretMove(ki.caretPos)
		}
		return CursorMoveEvent{Position: ki.caretPos}
	}

	return nil
}

// Layout registers the KeyInput for keyboard events.
// NOTE: Does NOT auto-focus. Call Focus(gtx) explicitly when the user
// clicks into a text zone, so top-bar editors can keep focus.
func (ki *KeyInput) Layout(gtx layout.Context) layout.Dimensions {
	key.InputHintOp{Tag: ki.focusTag, Hint: key.HintAny}.Add(gtx.Ops)
	event.Op(gtx.Ops, ki.focusTag)

	gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(500 * 1e6)})
	return layout.Dimensions{}
}

// Focus requests keyboard focus.
func (ki *KeyInput) Focus(gtx layout.Context) {
	gtx.Execute(key.FocusCmd{Tag: ki.focusTag})
}

// Focused reports if the widget has focus.
func (ki *KeyInput) Focused(gtx layout.Context) bool {
	return gtx.Focused(ki.focusTag)
}

// readCloser wraps a string as an io.ReadCloser for clipboard operations.
type readCloser struct {
	data string
	pos  int
}

func (r *readCloser) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *readCloser) Close() error {
	return nil
}
