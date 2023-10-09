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
	visible  bool
	new      bool
	hint     key.InputHint
	order    int
	dirOrder int
	filter   key.Set
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
	q.dirOrder = q.dirOrder[:0]
}

func (q *keyQueue) Frame(events *handlerEvents) {
	for k, h := range q.handlers {
		if !h.visible {
			delete(q.handlers, k)
			if q.focus == k {
				// Remove focus from the handler that is no longer visible.
				q.focus = nil
				q.state = TextInputClose
			}
		} else if h.new && k != q.focus {
			// Reset the handler on (each) first appearance, but don't trigger redraw.
			events.AddNoRedraw(k, key.FocusEvent{Focus: false})
		}
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
	return q.handlers[t].filter.Contains(e.Name, e.Modifiers)
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

func (q *keyQueue) handlerFor(tag event.Tag, area int, bounds image.Rectangle) *keyHandler {
	h, ok := q.handlers[tag]
	if !ok {
		h = &keyHandler{new: true, order: -1}
		q.handlers[tag] = h
	}
	if h.order == -1 {
		h.order = len(q.order)
		q.order = append(q.order, tag)
		q.dirOrder = append(q.dirOrder, dirFocusEntry{tag: tag, area: area, bounds: bounds})
	}
	return h
}

func (q *keyQueue) inputOp(op key.InputOp, area int, bounds image.Rectangle) {
	h := q.handlerFor(op.Tag, area, bounds)
	h.visible = true
	h.hint = op.Hint
	h.filter = op.Keys
}

func (q *keyQueue) selectionOp(t f32.Affine2D, op key.SelectionOp) {
	if op.Tag == q.focus {
		q.content.Selection.Range = op.Range
		q.content.Selection.Caret = op.Caret
		q.content.Selection.Transform = t
	}
}

func (q *keyQueue) snippetOp(op key.SnippetOp) {
	if op.Tag == q.focus {
		q.content.Snippet = op.Snippet
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
