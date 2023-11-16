// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"encoding/binary"
	"image"
	"io"
	"strings"
	"time"

	"gioui.org/f32"
	f32internal "gioui.org/internal/f32"
	"gioui.org/internal/ops"
	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/io/system"
	"gioui.org/io/transfer"
	"gioui.org/op"
)

// Router tracks the [io/event.Tag] identifiers of user interface widgets
// and routes events to them. [Source] is its interface exposed to widgets.
type Router struct {
	savedTrans []f32.Affine2D
	transStack []f32.Affine2D
	pointer    struct {
		queue     pointerQueue
		collector pointerCollector
	}
	key struct {
		queue keyQueue
	}
	cqueue clipboardQueue

	// states is the list of pending state changes resulting from
	// incoming events. The first element is the current state,
	// if any.
	changes []stateChange

	reader ops.Reader

	// InvalidateOp summary.
	wakeup     bool
	wakeupTime time.Time

	// Changes queued for next call to Frame.
	commands []Command

	// transfers is the pending transfer.DataEvent.Open functions.
	transfers []io.ReadCloser
}

// Source implements the interface between a Router and user interface widgets.
// The value Source is disabled.
type Source struct {
	r *Router
}

// Command represents a request such as moving the focus, or initiating a clipboard read.
// Commands are queued by calling [Source.Queue].
type Command interface {
	ImplementsCommand()
}

// SemanticNode represents a node in the tree describing the components
// contained in a frame.
type SemanticNode struct {
	ID       SemanticID
	ParentID SemanticID
	Children []SemanticNode
	Desc     SemanticDesc

	areaIdx int
}

// SemanticDesc provides a semantic description of a UI component.
type SemanticDesc struct {
	Class       semantic.ClassOp
	Description string
	Label       string
	Selected    bool
	Disabled    bool
	Gestures    SemanticGestures
	Bounds      image.Rectangle
}

// SemanticGestures is a bit-set of supported gestures.
type SemanticGestures int

const (
	ClickGesture SemanticGestures = 1 << iota
	ScrollGesture
)

// SemanticID uniquely identifies a SemanticDescription.
//
// By convention, the zero value denotes the non-existent ID.
type SemanticID uint

// stateChange represents the new state and outgoing events
// resulting from an incoming event.
type stateChange struct {
	state  inputState
	events []taggedEvent
}

// inputState represent a immutable snapshot of the state required
// to route events.
type inputState struct {
	clipboardState
	keyState
	pointerState
}

// taggedEvent represents an event and its target handler.
type taggedEvent struct {
	event event.Event
	tag   event.Tag
}

// Source returns a Source backed by this Router.
func (q *Router) Source() Source {
	return Source{r: q}
}

// Queue a command to be executed after the current frame
// has completed.
func (s Source) Queue(c Command) {
	if !s.Enabled() {
		return
	}
	s.r.queue(c)
}

// Enabled reports whether the source is enabled. Only enabled
// Sources deliver events and respond to commands.
func (s Source) Enabled() bool {
	return s.r != nil
}

// Events returns the events for the handler tag that matches one
// or more of filters.
func (s Source) Events(k event.Tag, filters ...event.Filter) []event.Event {
	if !s.Enabled() {
		return nil
	}
	return s.r.Events(k, filters...)
}

func (q *Router) Events(k event.Tag, filters ...event.Filter) []event.Event {
	var events []event.Event
	// Record handler filters and add reset events.
	for _, f := range filters {
		switch f := f.(type) {
		case key.Filter:
			q.key.queue.filter(k, f)
		case key.FocusFilter:
			q.key.queue.focusable(k)
			if reset, ok := q.key.queue.ResetEvent(k); ok {
				events = append(events, reset)
			}
		case pointer.Filter:
			q.pointer.queue.filterTag(k, f)
			if reset, ok := q.pointer.queue.ResetEvent(k); ok {
				events = append(events, reset)
			}
		case transfer.SourceFilter:
			q.pointer.queue.sourceFilter(k, f)
		case transfer.TargetFilter:
			q.pointer.queue.targetFilter(k, f)
		}
	}
	// Accumulate events from state changes until there are no more
	// matching events.
	matchedIdx := 0
	for i := range q.changes {
		change := &q.changes[i]
		j := 0
		for j < len(change.events) {
			evt := change.events[j]
			if evt.tag != k || !filtersMatches(filters, evt.event) {
				j++
				continue
			}
			events = append(events, evt.event)
			change.events = append(change.events[:j], change.events[j+1:]...)
			matchedIdx = i
		}
	}
	// Fast forward state to last matched.
	q.collapseState(matchedIdx)
	return events
}

// collapseState in the interval [1;idx] into q.changes[0].
func (q *Router) collapseState(idx int) {
	if idx == 0 {
		return
	}
	first := &q.changes[0]
	first.state = q.changes[idx].state
	for i := 1; i <= idx; i++ {
		first.events = append(first.events, q.changes[i].events...)
	}
	q.changes = append(q.changes[:1], q.changes[idx+1:]...)
}

// Frame replaces the declared handlers from the supplied
// operation list. The text input state, wakeup time and whether
// there are active profile handlers is also saved.
func (q *Router) Frame(frame *op.Ops) {
	for _, rc := range q.transfers {
		if rc != nil {
			rc.Close()
		}
	}
	q.transfers = nil
	q.wakeup = false
	// Collapse state and clear events.
	if n := len(q.changes); n > 1 {
		state := q.changes[n-1].state
		q.changes = append(q.changes[:0], stateChange{state: state})
	}
	var ops *ops.Ops
	if frame != nil {
		ops = &frame.Internal
	}
	q.reader.Reset(ops)
	q.collect()
	q.executeCommands()
	q.changePointerState(q.pointer.queue.Frame(q.lastState().pointerState))
	kstate := q.key.queue.Frame(q.lastState().keyState)
	q.changeKeyState(kstate, nil)
	// Collapse state and events.
	q.collapseState(len(q.changes) - 1)

	if len(q.changes) > 0 && len(q.changes[0].events) > 0 {
		q.wakeup = true
		q.wakeupTime = time.Time{}
	}
}

// Queue events and report whether at least one event matched a handler.
func (q *Router) Queue(events ...event.Event) bool {
	matched := false
	for _, e := range events {
		hadEvents := q.processEvent(e)
		matched = matched || hadEvents
	}
	return matched
}

func (q *Router) processEvent(e event.Event) bool {
	state := q.lastState()
	switch e := e.(type) {
	case pointer.Event:
		return q.changePointerState(q.pointer.queue.Push(state.pointerState, e))
	case key.Event:
		return q.addEvents(q.queueKeyEvent(state.keyState, e))
	case key.SnippetEvent:
		// Expand existing, overlapping snippet.
		if r := state.content.Snippet.Range; rangeOverlaps(r, key.Range(e)) {
			if e.Start > r.Start {
				e.Start = r.Start
			}
			if e.End < r.End {
				e.End = r.End
			}
		}
		var evts []taggedEvent
		if f := state.focus; f != nil {
			evts = append(evts, taggedEvent{tag: f, event: e})
		}
		return q.addEvents(evts)
	case key.EditEvent, key.FocusEvent, key.SelectionEvent:
		var evts []taggedEvent
		if f := state.focus; f != nil {
			evts = append(evts, taggedEvent{tag: f, event: e})
		}
		return q.addEvents(evts)
	case transfer.DataEvent:
		return q.changeClipboardState(q.cqueue.Push(state.clipboardState, e))
	default:
		panic("unknown event type")
	}
}

func (q *Router) queue(f Command) {
	q.commands = append(q.commands, f)
}

func (q *Router) state() inputState {
	if len(q.changes) > 0 {
		return q.changes[0].state
	}
	return inputState{}
}

func (q *Router) lastState() inputState {
	if n := len(q.changes); n > 0 {
		return q.changes[n-1].state
	}
	return inputState{}
}

func (q *Router) changeClipboardState(cstate clipboardState, evts []taggedEvent) bool {
	state := q.lastState()
	state.clipboardState = cstate
	return q.changeState(state, evts)
}

func (q *Router) changeKeyState(kstate keyState, evts []taggedEvent) bool {
	state := q.lastState()
	state.keyState = kstate
	return q.changeState(state, evts)
}

func (q *Router) changePointerState(pstate pointerState, evts []taggedEvent) bool {
	state := q.lastState()
	state.pointerState = pstate
	return q.changeState(state, evts)
}

func (q *Router) executeCommands() {
	for _, req := range q.commands {
		state := q.lastState()
		switch req := req.(type) {
		case key.SelectionCmd:
			kstate := q.key.queue.setSelection(state.keyState, req)
			q.changeKeyState(kstate, nil)
		case key.FocusCmd:
			q.changeKeyState(q.key.queue.Focus(state.keyState, req.Tag))
		case key.SoftKeyboardCmd:
			kstate := state.keyState.softKeyboard(req.Show)
			q.changeKeyState(kstate, nil)
		case key.SnippetCmd:
			kstate := q.key.queue.setSnippet(state.keyState, req)
			q.changeKeyState(kstate, nil)
		case transfer.OfferCmd:
			q.changePointerState(q.pointer.queue.offerData(state.pointerState, req))
		case clipboard.WriteCmd:
			q.cqueue.ProcessWriteClipboard(req)
		case clipboard.ReadCmd:
			cstate := q.cqueue.ProcessReadClipboard(state.clipboardState, req.Tag)
			q.changeClipboardState(cstate, nil)
		case pointer.GrabCmd:
			q.changePointerState(q.pointer.queue.grab(state.pointerState, req))
		}
	}
	q.commands = nil
}

func (q *Router) addEvents(evts []taggedEvent) bool {
	return q.changeState(q.lastState(), evts)
}

func (q *Router) changeState(state inputState, evts []taggedEvent) bool {
	// Wrap pointer.DataEvent.Open functions to detect them not being called.
	for i := range evts {
		e := &evts[i]
		if de, ok := e.event.(transfer.DataEvent); ok {
			transferIdx := len(q.transfers)
			data := de.Open()
			q.transfers = append(q.transfers, data)
			de.Open = func() io.ReadCloser {
				q.transfers[transferIdx] = nil
				return data
			}
			e.event = de
		}
	}
	n := len(q.changes)
	// We must add a new state change if
	//
	//  - there is no first state change, or
	//  - the state change is not atomic from the perspective of the handlers.
	if len(q.changes) == 0 || (len(evts) > 0 && len(q.changes[n-1].events) > 0) {
		q.changes = append(q.changes, stateChange{state: state, events: evts})
	} else {
		// Otherwise, merge with previous change.
		prev := &q.changes[n-1]
		prev.state = state
		prev.events = append(prev.events, evts...)
	}
	return len(evts) > 0
}

func rangeOverlaps(r1, r2 key.Range) bool {
	r1 = rangeNorm(r1)
	r2 = rangeNorm(r2)
	return r1.Start <= r2.Start && r2.Start < r1.End ||
		r1.Start <= r2.End && r2.End < r1.End
}

func rangeNorm(r key.Range) key.Range {
	if r.End < r.Start {
		r.End, r.Start = r.Start, r.End
	}
	return r
}

func (q *Router) queueKeyEvent(state keyState, e key.Event) []taggedEvent {
	kq := &q.key.queue
	f := state.focus
	var evts []taggedEvent
	if f != nil && kq.Accepts(f, e) {
		evts = append(evts, taggedEvent{tag: f, event: e})
		return evts
	}
	pq := &q.pointer.queue
	idx := len(pq.hitTree) - 1
	focused := f != nil
	if focused {
		// If there is a focused tag, traverse its ancestry through the
		// hit tree to search for handlers.
		for ; pq.hitTree[idx].tag != f; idx-- {
		}
	}
	for idx != -1 {
		n := &pq.hitTree[idx]
		if focused {
			idx = n.next
		} else {
			idx--
		}
		if n.tag == nil {
			continue
		}
		if kq.Accepts(n.tag, e) {
			evts = append(evts, taggedEvent{tag: n.tag, event: e})
			break
		}
	}
	return evts
}

func (q *Router) MoveFocus(dir key.FocusDirection) bool {
	ks, evts := q.key.queue.MoveFocus(q.lastState().keyState, dir)
	return q.changeKeyState(ks, evts)
}

// RevealFocus scrolls the current focus (if any) into viewport
// if there are scrollable parent handlers.
func (q *Router) RevealFocus(viewport image.Rectangle) {
	state := q.lastState()
	focus := state.focus
	if focus == nil {
		return
	}
	bounds := q.key.queue.BoundsFor(focus)
	area := q.key.queue.AreaFor(focus)
	viewport = q.pointer.queue.ClipFor(area, viewport)

	topleft := bounds.Min.Sub(viewport.Min)
	topleft = max(topleft, bounds.Max.Sub(viewport.Max))
	topleft = min(image.Pt(0, 0), topleft)
	bottomright := bounds.Max.Sub(viewport.Max)
	bottomright = min(bottomright, bounds.Min.Sub(viewport.Min))
	bottomright = max(image.Pt(0, 0), bottomright)
	s := topleft
	if s.X == 0 {
		s.X = bottomright.X
	}
	if s.Y == 0 {
		s.Y = bottomright.Y
	}
	q.ScrollFocus(s)
}

// ScrollFocus scrolls the focused widget, if any, by dist.
func (q *Router) ScrollFocus(dist image.Point) {
	state := q.lastState()
	focus := state.focus
	if focus == nil {
		return
	}
	area := q.key.queue.AreaFor(focus)
	q.addEvents(q.pointer.queue.Deliver(area, pointer.Event{
		Kind:   pointer.Scroll,
		Source: pointer.Touch,
		Scroll: f32internal.FPt(dist),
	}))
}

func max(p1, p2 image.Point) image.Point {
	m := p1
	if p2.X > m.X {
		m.X = p2.X
	}
	if p2.Y > m.Y {
		m.Y = p2.Y
	}
	return m
}

func min(p1, p2 image.Point) image.Point {
	m := p1
	if p2.X < m.X {
		m.X = p2.X
	}
	if p2.Y < m.Y {
		m.Y = p2.Y
	}
	return m
}

func (q *Router) ActionAt(p f32.Point) (system.Action, bool) {
	return q.pointer.queue.ActionAt(p)
}

func (q *Router) ClickFocus() {
	focus := q.lastState().focus
	if focus == nil {
		return
	}
	bounds := q.key.queue.BoundsFor(focus)
	center := bounds.Max.Add(bounds.Min).Div(2)
	e := pointer.Event{
		Position: f32.Pt(float32(center.X), float32(center.Y)),
		Source:   pointer.Touch,
	}
	area := q.key.queue.AreaFor(focus)
	e.Kind = pointer.Press
	q.addEvents(q.pointer.queue.Deliver(area, e))
	e.Kind = pointer.Release
	q.addEvents(q.pointer.queue.Deliver(area, e))
}

// TextInputState returns the input state from the most recent
// call to Frame.
func (q *Router) TextInputState() TextInputState {
	kstate, s := q.state().InputState()
	q.changeKeyState(kstate, nil)
	return s
}

// TextInputHint returns the input mode from the most recent key.InputOp.
func (q *Router) TextInputHint() (key.InputHint, bool) {
	return q.key.queue.InputHint(q.state().keyState)
}

// WriteClipboard returns the most recent content to be copied
// to the clipboard, if any.
func (q *Router) WriteClipboard() (mime string, content []byte, ok bool) {
	return q.cqueue.WriteClipboard()
}

// ClipboardRequested reports if any new handler is waiting
// to read the clipboard.
func (q *Router) ClipboardRequested() bool {
	return q.cqueue.ClipboardRequested(q.lastState().clipboardState)
}

// Cursor returns the last cursor set.
func (q *Router) Cursor() pointer.Cursor {
	return q.state().cursor
}

// SemanticAt returns the first semantic description under pos, if any.
func (q *Router) SemanticAt(pos f32.Point) (SemanticID, bool) {
	return q.pointer.queue.SemanticAt(pos)
}

// AppendSemantics appends the semantic tree to nodes, and returns the result.
// The root node is the first added.
func (q *Router) AppendSemantics(nodes []SemanticNode) []SemanticNode {
	q.pointer.collector.q = &q.pointer.queue
	q.pointer.collector.ensureRoot()
	return q.pointer.queue.AppendSemantics(nodes)
}

// EditorState returns the editor state for the focused handler, or the
// zero value if there is none.
func (q *Router) EditorState() EditorState {
	return q.key.queue.editorState(q.state().keyState)
}

func (q *Router) collect() {
	q.transStack = q.transStack[:0]
	pc := &q.pointer.collector
	pc.q = &q.pointer.queue
	pc.reset()
	kq := &q.key.queue
	q.key.queue.Reset()
	var t f32.Affine2D
	for encOp, ok := q.reader.Decode(); ok; encOp, ok = q.reader.Decode() {
		switch ops.OpType(encOp.Data[0]) {
		case ops.TypeInvalidate:
			op := decodeInvalidateOp(encOp.Data)
			if !q.wakeup || op.At.Before(q.wakeupTime) {
				q.wakeup = true
				q.wakeupTime = op.At
			}
		case ops.TypeSave:
			id := ops.DecodeSave(encOp.Data)
			if extra := id - len(q.savedTrans) + 1; extra > 0 {
				q.savedTrans = append(q.savedTrans, make([]f32.Affine2D, extra)...)
			}
			q.savedTrans[id] = t
		case ops.TypeLoad:
			id := ops.DecodeLoad(encOp.Data)
			t = q.savedTrans[id]
			pc.resetState()
			pc.setTrans(t)

		case ops.TypeClip:
			var op ops.ClipOp
			op.Decode(encOp.Data)
			pc.clip(op)
		case ops.TypePopClip:
			pc.popArea()
		case ops.TypeTransform:
			t2, push := ops.DecodeTransform(encOp.Data)
			if push {
				q.transStack = append(q.transStack, t)
			}
			t = t.Mul(t2)
			pc.setTrans(t)
		case ops.TypePopTransform:
			n := len(q.transStack)
			t = q.transStack[n-1]
			q.transStack = q.transStack[:n-1]
			pc.setTrans(t)

		case ops.TypeInput:
			tag := encOp.Refs[0].(event.Tag)
			pc.inputOp(tag)
			a := pc.currentArea()
			b := pc.currentAreaBounds()
			kq.inputOp(tag, t, a, b)

		// Pointer ops.
		case ops.TypePass:
			pc.pass()
		case ops.TypePopPass:
			pc.popPass()
		case ops.TypeCursor:
			name := pointer.Cursor(encOp.Data[1])
			pc.cursor(name)
		case ops.TypeActionInput:
			act := system.Action(encOp.Data[1])
			pc.actionInputOp(act)
		case ops.TypeKeyInputHint:
			op := key.InputHintOp{
				Tag:  encOp.Refs[0].(event.Tag),
				Hint: key.InputHint(encOp.Data[1]),
			}
			kq.inputHint(op)

		// Semantic ops.
		case ops.TypeSemanticLabel:
			lbl := *encOp.Refs[0].(*string)
			pc.semanticLabel(lbl)
		case ops.TypeSemanticDesc:
			desc := *encOp.Refs[0].(*string)
			pc.semanticDesc(desc)
		case ops.TypeSemanticClass:
			class := semantic.ClassOp(encOp.Data[1])
			pc.semanticClass(class)
		case ops.TypeSemanticSelected:
			if encOp.Data[1] != 0 {
				pc.semanticSelected(true)
			} else {
				pc.semanticSelected(false)
			}
		case ops.TypeSemanticEnabled:
			if encOp.Data[1] != 0 {
				pc.semanticEnabled(true)
			} else {
				pc.semanticEnabled(false)
			}
		}
	}
}

// WakeupTime returns the most recent time for doing another frame,
// as determined from the last call to Frame.
func (q *Router) WakeupTime() (time.Time, bool) {
	return q.wakeupTime, q.wakeup
}

func filtersMatches(filters []event.Filter, e event.Event) bool {
	switch e := e.(type) {
	case key.Event:
		for _, f := range filters {
			if f, ok := f.(key.Filter); ok {
				if keyFilterMatch(f, e) {
					return true
				}
			}
		}
	case key.FocusEvent, key.SnippetEvent, key.EditEvent, key.SelectionEvent:
		for _, f := range filters {
			if _, ok := f.(key.FocusFilter); ok {
				return true
			}
		}
	case pointer.Event:
		for _, f := range filters {
			if f, ok := f.(pointer.Filter); ok && f.Kinds&e.Kind == e.Kind {
				return true
			}
		}
	case transfer.CancelEvent, transfer.InitiateEvent:
		for _, f := range filters {
			switch f.(type) {
			case transfer.SourceFilter, transfer.TargetFilter:
				return true
			}
		}
	case transfer.RequestEvent:
		for _, f := range filters {
			if f, ok := f.(transfer.SourceFilter); ok && f.Type == e.Type {
				return true
			}
		}
	case transfer.DataEvent:
		for _, f := range filters {
			if f, ok := f.(transfer.TargetFilter); ok && f.Type == e.Type {
				return true
			}
		}
	}
	return false
}

func decodeInvalidateOp(d []byte) op.InvalidateOp {
	bo := binary.LittleEndian
	if ops.OpType(d[0]) != ops.TypeInvalidate {
		panic("invalid op")
	}
	var o op.InvalidateOp
	if nanos := bo.Uint64(d[1:]); nanos > 0 {
		o.At = time.Unix(0, int64(nanos))
	}
	return o
}

func (s SemanticGestures) String() string {
	var gestures []string
	if s&ClickGesture != 0 {
		gestures = append(gestures, "Click")
	}
	return strings.Join(gestures, ",")
}
