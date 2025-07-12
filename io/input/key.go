// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"image"
	"slices"
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
	reset        bool
	hint         key.InputHint
	orderPlusOne int
	dirOrder     int
	trans        f32.Affine2D
}

type keyFilter []key.Filter

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

func (k *keyHandler) inputHint(hint key.InputHint) {
	k.hint = hint
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
func (q *keyQueue) InputHint(handlers map[event.Tag]*handler, state keyState) (key.InputHint, bool) {
	focused, ok := handlers[state.focus]
	if !ok {
		return q.hint, false
	}
	old := q.hint
	q.hint = focused.key.hint
	return q.hint, old != q.hint
}

func (k *keyHandler) Reset() {
	k.visible = false
	k.orderPlusOne = 0
	k.hint = key.HintAny
}

func (q *keyQueue) Reset() {
	q.order = q.order[:0]
	q.dirOrder = q.dirOrder[:0]
}

func (k *keyHandler) ResetEvent() (event.Event, bool) {
	if k.reset {
		return nil, false
	}
	k.reset = true
	return key.FocusEvent{Focus: false}, true
}

func (q *keyQueue) Frame(handlers map[event.Tag]*handler, state keyState) keyState {
	if state.focus != nil {
		if h, ok := handlers[state.focus]; !ok || !h.filter.focusable || !h.key.visible {
			// Remove focus from the handler that is no longer focusable.
			state.focus = nil
			state.state = TextInputClose
		}
	}
	q.updateFocusLayout(handlers)
	return state
}

// updateFocusLayout partitions input handlers handlers into rows
// for directional focus moves.
//
// The approach is greedy: pick the topmost handler and create a row
// containing it. Then, extend the handler bounds to a horizontal beam
// and add to the row every handler whose center intersect it. Repeat
// until no handlers remain.
func (q *keyQueue) updateFocusLayout(handlers map[event.Tag]*handler) {
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
		handlers[o.tag].key.dirOrder = i
	}
}

// MoveFocus attempts to move the focus in the direction of dir.
func (q *keyQueue) MoveFocus(handlers map[event.Tag]*handler, state keyState, dir key.FocusDirection) (keyState, []taggedEvent) {
	if len(q.dirOrder) == 0 {
		return state, nil
	}
	order := 0
	if state.focus != nil {
		order = handlers[state.focus].key.dirOrder
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
			order = handlers[state.focus].key.orderPlusOne - 1
			if dir == key.FocusForward {
				order++
			} else {
				order--
			}
		}
		order = (order + len(q.order)) % len(q.order)
		return q.Focus(handlers, state, q.order[order])
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
				return q.Focus(handlers, state, newFocus.tag)
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
			return q.Focus(handlers, state, closest)
		}
	}
	return state, nil
}

func (q *keyQueue) BoundsFor(k *keyHandler) image.Rectangle {
	order := k.dirOrder
	return q.dirOrder[order].bounds
}

func (q *keyQueue) AreaFor(k *keyHandler) int {
	order := k.dirOrder
	return q.dirOrder[order].area
}

func (k *keyFilter) Matches(focus event.Tag, e key.Event, system bool) bool {
	for _, f := range *k {
		if keyFilterMatch(focus, f, e, system) {
			return true
		}
	}
	return false
}

func keyFilterMatch(focus event.Tag, f key.Filter, e key.Event, system bool) bool {
	if f.Focus != nil && f.Focus != focus {
		return false
	}
	if (f.Name != "" || system) && f.Name != e.Name {
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

func (q *keyQueue) Focus(handlers map[event.Tag]*handler, state keyState, focus event.Tag) (keyState, []taggedEvent) {
	if focus == state.focus {
		return state, nil
	}
	state.content = EditorState{}
	state.content.Selection.Transform = f32.AffineId()
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

func (k *keyFilter) Add(f key.Filter) {
	if slices.Contains(*k, f) {
		return
	}
	*k = append(*k, f)
}

func (k *keyFilter) Merge(k2 keyFilter) {
	*k = append(*k, k2...)
}

func (q *keyQueue) inputOp(tag event.Tag, state *keyHandler, t f32.Affine2D, area int, bounds image.Rectangle) {
	state.visible = true
	if state.orderPlusOne == 0 {
		state.orderPlusOne = len(q.order) + 1
		q.order = append(q.order, tag)
		q.dirOrder = append(q.dirOrder, dirFocusEntry{tag: tag, area: area, bounds: bounds})
	}
	state.trans = t
}

func (q *keyQueue) setSelection(state keyState, req key.SelectionCmd) keyState {
	if req.Tag != state.focus {
		return state
	}
	state.content.Selection.Range = req.Range
	state.content.Selection.Caret = req.Caret
	return state
}

func (q *keyQueue) editorState(handlers map[event.Tag]*handler, state keyState) EditorState {
	s := state.content
	if f := state.focus; f != nil {
		s.Selection.Transform = handlers[f].key.trans
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
