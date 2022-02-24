// SPDX-License-Identifier: Unlicense OR MIT

package router

import (
	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
)

// EditorState represents the state of an editor needed by input handlers.
type EditorState struct {
	Selection struct {
		Transform f32.Affine2D
		key.Range
		key.Caret
	}
	Snippet key.Snippet
}

type TextInputState uint8

type keyQueue struct {
	focus    event.Tag
	order    []event.Tag
	handlers map[event.Tag]*keyHandler
	state    TextInputState
	hint     key.InputHint
	content  EditorState
}

type keyHandler struct {
	// visible will be true if the InputOp is present
	// in the current frame.
	visible bool
	new     bool
	hint    key.InputHint
	order   int
}

// keyCollector tracks state required to update a keyQueue
// from key ops.
type keyCollector struct {
	q       *keyQueue
	focus   event.Tag
	changed bool
}

const (
	TextInputKeep TextInputState = iota
	TextInputClose
	TextInputOpen
)

// InputState returns the last text input state as
// determined in Frame.
func (q *keyQueue) InputState() TextInputState {
	state := q.state
	q.state = TextInputKeep
	return state
}

// InputHint returns the input mode from the most recent key.InputOp.
func (q *keyQueue) InputHint() (key.InputHint, bool) {
	if q.focus == nil {
		return q.hint, false
	}
	focused, ok := q.handlers[q.focus]
	if !ok {
		return q.hint, false
	}
	old := q.hint
	q.hint = focused.hint
	return q.hint, old != q.hint
}

func (q *keyQueue) Reset() {
	if q.handlers == nil {
		q.handlers = make(map[event.Tag]*keyHandler)
	}
	for _, h := range q.handlers {
		h.visible, h.new = false, false
		h.order = -1
	}
	q.order = q.order[:0]
}

func (q *keyQueue) Frame(events *handlerEvents, collector keyCollector) {
	changed, focus := collector.changed, collector.focus
	for k, h := range q.handlers {
		if !h.visible {
			delete(q.handlers, k)
			if q.focus == k {
				// Remove focus from the handler that is no longer visible.
				q.focus = nil
				q.state = TextInputClose
			}
		} else if h.new && k != focus {
			// Reset the handler on (each) first appearance, but don't trigger redraw.
			events.AddNoRedraw(k, key.FocusEvent{Focus: false})
		}
	}
	if changed {
		q.setFocus(focus, events)
	}
}

func (q *keyQueue) Push(e event.Event, events *handlerEvents) {
	// Convert tab or shift+tab presses to focus moves.
	if e, ok := e.(key.Event); ok && e.Name == key.NameTab && e.Modifiers&^key.ModShift == 0 {
		if e.State == key.Release || len(q.order) == 0 {
			return
		}
		forward := !e.Modifiers.Contain(key.ModShift)
		order := 0
		if !forward {
			order = -1
		}
		if q.focus != nil {
			order = q.handlers[q.focus].order
			if forward {
				order++
			} else {
				order--
			}
		}
		order = (order + len(q.order)) % len(q.order)
		q.setFocus(q.order[order], events)
		return
	}
	if q.focus != nil {
		events.Add(q.focus, e)
	}
}

func (q *keyQueue) setFocus(focus event.Tag, events *handlerEvents) {
	if focus != nil {
		if _, exists := q.handlers[focus]; !exists {
			focus = nil
		}
	}
	if focus == q.focus {
		return
	}
	q.content = EditorState{}
	if q.focus != nil {
		events.Add(q.focus, key.FocusEvent{Focus: false})
	}
	q.focus = focus
	if q.focus != nil {
		events.Add(q.focus, key.FocusEvent{Focus: true})
	}
	if q.focus == nil || q.state == TextInputKeep {
		q.state = TextInputClose
	}
}

func (k *keyCollector) focusOp(tag event.Tag) {
	k.focus = tag
	k.changed = true
}

func (k *keyCollector) softKeyboard(show bool) {
	if show {
		k.q.state = TextInputOpen
	} else {
		k.q.state = TextInputClose
	}
}

func (k *keyCollector) handlerFor(tag event.Tag) *keyHandler {
	h, ok := k.q.handlers[tag]
	if !ok {
		h = &keyHandler{new: true, order: -1}
		k.q.handlers[tag] = h
	}
	if h.order == -1 {
		h.order = len(k.q.order)
		k.q.order = append(k.q.order, tag)
	}
	return h
}

func (k *keyCollector) inputOp(op key.InputOp) {
	h := k.handlerFor(op.Tag)
	h.visible = true
	h.hint = op.Hint
}

func (k *keyCollector) selectionOp(t f32.Affine2D, op key.SelectionOp) {
	if op.Tag == k.q.focus {
		k.q.content.Selection.Range = op.Range
		k.q.content.Selection.Caret = op.Caret
		k.q.content.Selection.Transform = t
	}
}

func (k *keyCollector) snippetOp(op key.SnippetOp) {
	if op.Tag == k.q.focus {
		k.q.content.Snippet = op.Snippet
	}
}

func (t TextInputState) String() string {
	switch t {
	case TextInputKeep:
		return "Keep"
	case TextInputClose:
		return "Close"
	case TextInputOpen:
		return "Open"
	default:
		panic("unexpected value")
	}
}
