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
	focus    event.Tag
	order    []event.Tag
	dirOrder []dirFocusEntry
	handlers map[event.Tag]*keyHandler
	state    TextInputState
	hint     key.InputHint
	content  EditorState
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

// InputState returns the last text input state as
// determined in Frame.
func (q *keyQueue) InputState() TextInputState {
	state := q.state
	q.state = TextInputKeep
	return state
}

// InputHint returns the input hint from the focused handler and whether it was
// changed since the last call.
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

func (q *keyQueue) Frame() {
	for k, h := range q.handlers {
		if !h.visible || !h.focusable {
			if q.focus == k {
				// Remove focus from the handler that is no longer focusable.
				q.focus = nil
				q.state = TextInputClose
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

// MoveFocus attempts to move the focus in the direction of dir, returning true if it succeeds.
func (q *keyQueue) MoveFocus(dir key.FocusDirection, events *handlerEvents) bool {
	if len(q.dirOrder) == 0 {
		return false
	}
	order := 0
	if q.focus != nil {
		order = q.handlers[q.focus].dirOrder
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
		if q.focus != nil {
			order = q.handlers[q.focus].order
			if dir == key.FocusForward {
				order++
			} else {
				order--
			}
		}
		order = (order + len(q.order)) % len(q.order)
		q.Focus(q.order[order], events)
		return true
	case key.FocusRight, key.FocusLeft:
		next := order
		if q.focus != nil {
			next = order + 1
			if dir == key.FocusLeft {
				next = order - 1
			}
		}
		if 0 <= next && next < len(q.dirOrder) {
			newFocus := q.dirOrder[next]
			if newFocus.row == focus.row {
				q.Focus(newFocus.tag, events)
				return true
			}
		}
	case key.FocusUp, key.FocusDown:
		delta := +1
		if dir == key.FocusUp {
			delta = -1
		}
		nextRow := 0
		if q.focus != nil {
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
			q.Focus(closest, events)
			return true
		}
	}
	return false
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

func (q *keyQueue) Focus(focus event.Tag, events *handlerEvents) {
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

func (q *keyQueue) softKeyboard(show bool) {
	if show {
		q.state = TextInputOpen
	} else {
		q.state = TextInputClose
	}
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

func (q *keyQueue) setSelection(req key.SelectionCmd) {
	if req.Tag != q.focus {
		return
	}
	q.content.Selection.Range = req.Range
	q.content.Selection.Caret = req.Caret
}

func (q *keyQueue) editorState() EditorState {
	s := q.content
	if f := q.focus; f != nil {
		s.Selection.Transform = q.handlers[f].trans
	}
	return s
}

func (q *keyQueue) setSnippet(req key.SnippetCmd) {
	if req.Tag == q.focus {
		q.content.Snippet = req.Snippet
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
