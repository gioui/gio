// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"image"
	"sort"

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
	order    []event.Tag
	dirOrder []dirFocusEntry
	handlers map[event.Tag]*keyHandler
	hint     key.InputHint
}

// keyState is the input state related to key events.
type keyState struct {
	focus   event.Tag
	state   TextInputState
	content EditorState
}

type keyHandler struct {
	// visible will be true if the InputOp is present
	// in the current frame.
	visible bool
	// reset tracks whether the handler has seen a
	// focus reset.
	reset     bool
	focusable bool
	active    bool
	hint      key.InputHint
	order     int
	dirOrder  int
	filters   []key.Filter
	trans     f32.Affine2D
}

type dirFocusEntry struct {
	tag    event.Tag
	row    int
	area   int
	bounds image.Rectangle
}

const (
	TextInputKeep TextInputState = iota
	TextInputClose
	TextInputOpen
)

func (q *keyQueue) inputHint(op key.InputHintOp) {
	h := q.handlerFor(op.Tag)
	h.hint = op.Hint
}

// InputState returns the input state and returns a state
// reset to [TextInputKeep].
func (s keyState) InputState() (keyState, TextInputState) {
	state := s.state
	s.state = TextInputKeep
	return s, state
}

// InputHint returns the input hint from the focused handler and whether it was
// changed since the last call.
func (q *keyQueue) InputHint(state keyState) (key.InputHint, bool) {
	focused, ok := q.handlers[state.focus]
	if !ok {
		return q.hint, false
	}
	old := q.hint
	q.hint = focused.hint
	return q.hint, old != q.hint
}

func (q *keyQueue) Reset() {
	for _, h := range q.handlers {
		h.order = -1
		h.hint = key.HintAny
	}
	q.order = q.order[:0]
	q.dirOrder = q.dirOrder[:0]
}

func (q *keyQueue) ResetEvent(k event.Tag) (event.Event, bool) {
	h, ok := q.handlers[k]
	if !ok || h.reset {
		return nil, false
	}
	h.reset = true
	return key.FocusEvent{Focus: false}, true
}

func (q *keyQueue) Frame(state keyState) keyState {
	for k, h := range q.handlers {
		if !h.visible || !h.focusable {
			if state.focus == k {
				// Remove focus from the handler that is no longer focusable.
				state.focus = nil
				state.state = TextInputClose
			}
			if !h.visible && !h.focusable {
				delete(q.handlers, k)
				continue
			}
		}
		h.visible = false
		h.focusable = false
		h.active = false
	}
	q.updateFocusLayout()
	return state
}

// updateFocusLayout partitions input handlers handlers into rows
// for directional focus moves.
//
// The approach is greedy: pick the topmost handler and create a row
// containing it. Then, extend the handler bounds to a horizontal beam
// and add to the row every handler whose center intersect it. Repeat
// until no handlers remain.
func (q *keyQueue) updateFocusLayout() {
	order := q.dirOrder
	// Sort by ascending y position.
	sort.SliceStable(order, func(i, j int) bool {
		return order[i].bounds.Min.Y < order[j].bounds.Min.Y
	})
	row := 0
	for len(order) > 0 {
		h := &order[0]
		h.row = row
		bottom := h.bounds.Max.Y
		end := 1
		for ; end < len(order); end++ {
			h := &order[end]
			center := (h.bounds.Min.Y + h.bounds.Max.Y) / 2
			if center > bottom {
				break
			}
			h.row = row
		}
		// Sort row by ascending x position.
		sort.SliceStable(order[:end], func(i, j int) bool {
			return order[i].bounds.Min.X < order[j].bounds.Min.X
		})
		order = order[end:]
		row++
	}
	for i, o := range q.dirOrder {
		q.handlers[o.tag].dirOrder = i
	}
}

// MoveFocus attempts to move the focus in the direction of dir.
func (q *keyQueue) MoveFocus(state keyState, dir key.FocusDirection) (keyState, []taggedEvent) {
	if len(q.dirOrder) == 0 {
		return state, nil
	}
	order := 0
	if state.focus != nil {
		order = q.handlers[state.focus].dirOrder
	}
	focus := q.dirOrder[order]
	switch dir {
	case key.FocusForward, key.FocusBackward:
		if len(q.order) == 0 {
			break
		}
		order := 0
		if dir == key.FocusBackward {
			order = -1
		}
		if state.focus != nil {
			order = q.handlers[state.focus].order
			if dir == key.FocusForward {
				order++
			} else {
				order--
			}
		}
		order = (order + len(q.order)) % len(q.order)
		return q.Focus(state, q.order[order])
	case key.FocusRight, key.FocusLeft:
		next := order
		if state.focus != nil {
			next = order + 1
			if dir == key.FocusLeft {
				next = order - 1
			}
		}
		if 0 <= next && next < len(q.dirOrder) {
			newFocus := q.dirOrder[next]
			if newFocus.row == focus.row {
				return q.Focus(state, newFocus.tag)
			}
		}
	case key.FocusUp, key.FocusDown:
		delta := +1
		if dir == key.FocusUp {
			delta = -1
		}
		nextRow := 0
		if state.focus != nil {
			nextRow = focus.row + delta
		}
		var closest event.Tag
		dist := int(1e6)
		center := (focus.bounds.Min.X + focus.bounds.Max.X) / 2
	loop:
		for 0 <= order && order < len(q.dirOrder) {
			next := q.dirOrder[order]
			switch next.row {
			case nextRow:
				nextCenter := (next.bounds.Min.X + next.bounds.Max.X) / 2
				d := center - nextCenter
				if d < 0 {
					d = -d
				}
				if d > dist {
					break loop
				}
				dist = d
				closest = next.tag
			case nextRow + delta:
				break loop
			}
			order += delta
		}
		if closest != nil {
			return q.Focus(state, closest)
		}
	}
	return state, nil
}

func (q *keyQueue) BoundsFor(t event.Tag) image.Rectangle {
	order := q.handlers[t].dirOrder
	return q.dirOrder[order].bounds
}

func (q *keyQueue) AreaFor(t event.Tag) int {
	order := q.handlers[t].dirOrder
	return q.dirOrder[order].area
}

func (q *keyQueue) Accepts(t event.Tag, e key.Event) bool {
	for _, f := range q.handlers[t].filters {
		if keyFilterMatch(f, e) {
			return true
		}
	}
	return false
}

func keyFilterMatch(f key.Filter, e key.Event) bool {
	if f.Name != e.Name {
		return false
	}
	if e.Modifiers&f.Required != f.Required {
		return false
	}
	if e.Modifiers&^(f.Required|f.Optional) != 0 {
		return false
	}
	return true
}

func (q *keyQueue) Focus(state keyState, focus event.Tag) (keyState, []taggedEvent) {
	if focus != nil {
		if _, exists := q.handlers[focus]; !exists {
			focus = nil
		}
	}
	if focus == state.focus {
		return state, nil
	}
	state.content = EditorState{}
	var evts []taggedEvent
	if state.focus != nil {
		evts = append(evts, taggedEvent{tag: state.focus, event: key.FocusEvent{Focus: false}})
	}
	state.focus = focus
	if state.focus != nil {
		evts = append(evts, taggedEvent{tag: state.focus, event: key.FocusEvent{Focus: true}})
	}
	if state.focus == nil || state.state == TextInputKeep {
		state.state = TextInputClose
	}
	return state, evts
}

func (s keyState) softKeyboard(show bool) keyState {
	if show {
		s.state = TextInputOpen
	} else {
		s.state = TextInputClose
	}
	return s
}

func (q *keyQueue) filter(tag event.Tag, f key.Filter) {
	h := q.handlerFor(tag)
	if !h.active {
		h.active = true
		h.filters = h.filters[:0]
	}
	h.filters = append(h.filters, f)
}

func (q *keyQueue) focusable(tag event.Tag) {
	h := q.handlerFor(tag)
	h.focusable = true
}

func (q *keyQueue) handlerFor(tag event.Tag) *keyHandler {
	h, ok := q.handlers[tag]
	if !ok {
		h = &keyHandler{order: -1}
		if q.handlers == nil {
			q.handlers = make(map[event.Tag]*keyHandler)
		}
		q.handlers[tag] = h
	}
	return h
}

func (q *keyQueue) inputOp(tag event.Tag, t f32.Affine2D, area int, bounds image.Rectangle) {
	h := q.handlerFor(tag)
	if h.order == -1 {
		h.order = len(q.order)
		q.order = append(q.order, tag)
		q.dirOrder = append(q.dirOrder, dirFocusEntry{tag: tag, area: area, bounds: bounds})
	}
	h.visible = true
	h.trans = t
}

func (q *keyQueue) setSelection(state keyState, req key.SelectionCmd) keyState {
	if req.Tag != state.focus {
		return state
	}
	state.content.Selection.Range = req.Range
	state.content.Selection.Caret = req.Caret
	return state
}

func (q *keyQueue) editorState(state keyState) EditorState {
	s := state.content
	if f := state.focus; f != nil {
		s.Selection.Transform = q.handlers[f].trans
	}
	return s
}

func (q *keyQueue) setSnippet(state keyState, req key.SnippetCmd) keyState {
	if req.Tag == state.focus {
		state.content.Snippet = req.Snippet
	}
	return state
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
