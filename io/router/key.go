// SPDX-License-Identifier: Unlicense OR MIT

package router

import (
	"gioui.org/internal/opconst"
	"gioui.org/internal/ops"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/op"
)

type TextInputState uint8

type keyQueue struct {
	focus    event.Tag
	handlers map[event.Tag]*keyHandler
	reader   ops.Reader
	state    TextInputState
	// states store states during resolveFocus
	states []resolveState
}

type resolveState struct {
	tag      event.Tag
	pri      listenerPriority
	keyboard TextInputState
}

type keyHandler struct {
	// visible will be true if the InputOp is present
	// in the current frame.
	visible bool
	new     bool
}

type listenerPriority uint8

const (
	priDefault listenerPriority = iota
	priCurrentFocus
	priNone
	priNewFocus
)

const (
	TextInputKeep TextInputState = iota
	TextInputClose
	TextInputOpen
)

// InputState returns the last text input state as
// determined in Frame.
func (q *keyQueue) InputState() TextInputState {
	return q.state
}

func (q *keyQueue) Frame(root *op.Ops, events *handlerEvents) {
	if q.handlers == nil {
		q.handlers = make(map[event.Tag]*keyHandler)
	}
	for _, h := range q.handlers {
		h.visible, h.new = false, false
	}
	q.reader.Reset(root)

	state := q.resolveFocus(events)
	if state.pri == priNone {
		state.tag = nil
	}
	for k, h := range q.handlers {
		if !h.visible {
			delete(q.handlers, k)
			if q.focus == k {
				// Remove the focus from the handler that is no longer visible.
				q.focus = nil
				state.keyboard = TextInputClose
			}
		}
		if h.new && k != state.tag {
			// Reset the handler on (each) first appearance.
			events.Add(k, key.FocusEvent{Focus: false})
		}
	}
	if state.tag != q.focus {
		if q.focus != nil {
			events.Add(q.focus, key.FocusEvent{Focus: false})
		}
		q.focus = state.tag
		if q.focus != nil {
			events.Add(q.focus, key.FocusEvent{Focus: true})
		} else {
			state.keyboard = TextInputClose
		}
	}
	q.state = state.keyboard
}

func (q *keyQueue) Push(e event.Event, events *handlerEvents) {
	if q.focus != nil {
		events.Add(q.focus, e)
	}
}

func (q *keyQueue) resolveFocus(events *handlerEvents) resolveState {
	var state resolveState
	for encOp, ok := q.reader.Decode(); ok; encOp, ok = q.reader.Decode() {
		switch opconst.OpType(encOp.Data[0]) {
		case opconst.TypeKeyFocus:
			op := decodeFocusOp(encOp.Data, encOp.Refs)
			if op.Focus {
				state.pri = priNewFocus
			} else {
				state.pri, state.keyboard = priNone, TextInputClose
			}
		case opconst.TypeKeySoftKeyboard:
			op := decodeSoftKeyboardOp(encOp.Data, encOp.Refs)
			if op.Show {
				state.keyboard = TextInputOpen
			} else {
				state.keyboard = TextInputClose
			}
		case opconst.TypeKeyInput:
			op := decodeKeyInputOp(encOp.Data, encOp.Refs)
			if op.Tag == q.focus && state.pri < priCurrentFocus {
				state.pri = priCurrentFocus
			}
			h, ok := q.handlers[op.Tag]
			if !ok {
				h = &keyHandler{new: true}
				q.handlers[op.Tag] = h
			}
			h.visible = true
			state.tag = op.Tag
		case opconst.TypeSave:
			id := ops.DecodeSave(encOp.Data)
			if extra := id - len(q.states) + 1; extra > 0 {
				q.states = append(q.states, make([]resolveState, extra)...)
			}
			q.states[id] = state
			state = resolveState{}
		case opconst.TypeLoad:
			id, mask := ops.DecodeLoad(encOp.Data)
			restored := q.states[id]
			if state.keyboard > restored.keyboard {
				restored.keyboard = state.keyboard
			}
			if state.pri.replaces(restored.pri) {
				restored.tag, restored.pri = state.tag, state.pri
			}
			if mask != 0 {
				state = restored
			}
		}
	}
	return state
}

func (p listenerPriority) replaces(p2 listenerPriority) bool {
	// Favor earliest default focus or latest requested focus.
	return p > p2 || p == p2 && p == priNewFocus
}

func decodeKeyInputOp(d []byte, refs []interface{}) key.InputOp {
	if opconst.OpType(d[0]) != opconst.TypeKeyInput {
		panic("invalid op")
	}
	return key.InputOp{
		Tag: refs[0].(event.Tag),
	}
}

func decodeSoftKeyboardOp(d []byte, refs []interface{}) key.SoftKeyboardOp {
	if opconst.OpType(d[0]) != opconst.TypeKeySoftKeyboard {
		panic("invalid op")
	}
	return key.SoftKeyboardOp{
		Show: d[1] != 0,
	}
}

func decodeFocusOp(d []byte, refs []interface{}) key.FocusOp {
	if opconst.OpType(d[0]) != opconst.TypeKeyFocus {
		panic("invalid op")
	}
	return key.FocusOp{
		Focus: d[1] != 0,
	}
}
