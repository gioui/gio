// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"encoding/binary"
	"image"
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

	handlers handlerEvents

	reader ops.Reader

	// InvalidateOp summary.
	wakeup     bool
	wakeupTime time.Time

	// Changes queued for next call to Frame.
	commands []Command
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

type handlerEvents struct {
	handlers  map[event.Tag][]event.Event
	hadEvents bool
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
	var resetEvents []event.Event
	for _, f := range filters {
		switch f := f.(type) {
		case key.Filter:
			q.key.queue.filter(k, f)
		case key.FocusFilter:
			q.key.queue.focusable(k)
			if reset, ok := q.key.queue.ResetEvent(k); ok {
				resetEvents = append(resetEvents, reset)
			}
		case pointer.Filter:
			q.pointer.queue.filterTag(k, f)
			if reset, ok := q.pointer.queue.ResetEvent(k); ok {
				resetEvents = append(resetEvents, reset)
			}
		case transfer.SourceFilter:
			q.pointer.queue.sourceFilter(k, f)
		case transfer.TargetFilter:
			q.pointer.queue.targetFilter(k, f)
		}
	}
	events := q.handlers.Events(k, filters...)
	return append(resetEvents, events...)
}

// Frame replaces the declared handlers from the supplied
// operation list. The text input state, wakeup time and whether
// there are active profile handlers is also saved.
func (q *Router) Frame(frame *op.Ops) {
	q.handlers.Clear()
	q.wakeup = false
	var ops *ops.Ops
	if frame != nil {
		ops = &frame.Internal
	}
	q.reader.Reset(ops)
	q.collect()
	q.executeCommands()
	q.pointer.queue.Frame(&q.handlers)
	q.key.queue.Frame()

	if q.handlers.HadEvents() {
		q.wakeup = true
		q.wakeupTime = time.Time{}
	}
}

// Queue events and report whether at least one handler had an event queued.
func (q *Router) Queue(events ...event.Event) bool {
	for _, e := range events {
		switch e := e.(type) {
		case pointer.Event:
			q.pointer.queue.Push(e, &q.handlers)
		case key.Event:
			q.queueKeyEvent(e)
		case key.SnippetEvent:
			// Expand existing, overlapping snippet.
			if r := q.key.queue.content.Snippet.Range; rangeOverlaps(r, key.Range(e)) {
				if e.Start > r.Start {
					e.Start = r.Start
				}
				if e.End < r.End {
					e.End = r.End
				}
			}
			if f := q.key.queue.focus; f != nil {
				q.handlers.Add(f, e)
			}
		case key.EditEvent, key.FocusEvent, key.SelectionEvent:
			if f := q.key.queue.focus; f != nil {
				q.handlers.Add(f, e)
			}
		case transfer.DataEvent:
			q.cqueue.Push(e, &q.handlers)
		}
	}
	return q.handlers.HadEvents()
}

func (q *Router) queue(f Command) {
	q.commands = append(q.commands, f)
}

func (q *Router) executeCommands() {
	for _, req := range q.commands {
		switch req := req.(type) {
		case key.SelectionCmd:
			q.key.queue.setSelection(req)
		case key.FocusCmd:
			q.key.queue.Focus(req.Tag, &q.handlers)
		case key.SoftKeyboardCmd:
			q.key.queue.softKeyboard(req.Show)
		case key.SnippetCmd:
			q.key.queue.setSnippet(req)
		case transfer.OfferCmd:
			q.pointer.queue.offerData(req, &q.handlers)
		case clipboard.WriteCmd:
			q.cqueue.ProcessWriteClipboard(req)
		case clipboard.ReadCmd:
			q.cqueue.ProcessReadClipboard(req.Tag)
		case pointer.GrabCmd:
			q.pointer.queue.grab(req, &q.handlers)
		}
	}
	q.commands = nil
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

func (q *Router) queueKeyEvent(e key.Event) {
	kq := &q.key.queue
	f := q.key.queue.focus
	if f != nil && kq.Accepts(f, e) {
		q.handlers.Add(f, e)
		return
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
			q.handlers.Add(n.tag, e)
			break
		}
	}
}

func (q *Router) MoveFocus(dir key.FocusDirection) bool {
	return q.key.queue.MoveFocus(dir, &q.handlers)
}

// RevealFocus scrolls the current focus (if any) into viewport
// if there are scrollable parent handlers.
func (q *Router) RevealFocus(viewport image.Rectangle) {
	focus := q.key.queue.focus
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
	focus := q.key.queue.focus
	if focus == nil {
		return
	}
	area := q.key.queue.AreaFor(focus)
	q.pointer.queue.Deliver(area, pointer.Event{
		Kind:   pointer.Scroll,
		Source: pointer.Touch,
		Scroll: f32internal.FPt(dist),
	}, &q.handlers)
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
	focus := q.key.queue.focus
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
	q.pointer.queue.Deliver(area, e, &q.handlers)
	e.Kind = pointer.Release
	q.pointer.queue.Deliver(area, e, &q.handlers)
}

// TextInputState returns the input state from the most recent
// call to Frame.
func (q *Router) TextInputState() TextInputState {
	return q.key.queue.InputState()
}

// TextInputHint returns the input mode from the most recent key.InputOp.
func (q *Router) TextInputHint() (key.InputHint, bool) {
	return q.key.queue.InputHint()
}

// WriteClipboard returns the most recent content to be copied
// to the clipboard, if any.
func (q *Router) WriteClipboard() (mime string, content []byte, ok bool) {
	return q.cqueue.WriteClipboard()
}

// ReadClipboard reports if any new handler is waiting
// to read the clipboard.
func (q *Router) ReadClipboard() bool {
	return q.cqueue.ReadClipboard()
}

// Cursor returns the last cursor set.
func (q *Router) Cursor() pointer.Cursor {
	return q.pointer.queue.cursor
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
	return q.key.queue.editorState()
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

func (h *handlerEvents) init() {
	if h.handlers == nil {
		h.handlers = make(map[event.Tag][]event.Event)
	}
}

func (h *handlerEvents) Add(k event.Tag, e event.Event) {
	h.init()
	h.handlers[k] = append(h.handlers[k], e)
	h.hadEvents = true
}

func (h *handlerEvents) HadEvents() bool {
	u := h.hadEvents
	h.hadEvents = false
	return u
}

func (h *handlerEvents) Events(k event.Tag, filters ...event.Filter) []event.Event {
	var filtered []event.Event
	if events, ok := h.handlers[k]; ok {
		i := 0
		for i < len(events) {
			e := events[i]
			if filtersMatches(filters, e) {
				filtered = append(filtered, e)
				events = append(events[:i], events[i+1:]...)
			} else {
				i++
			}
		}
		h.handlers[k] = events
	}
	return filtered
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

func (h *handlerEvents) Clear() {
	for k := range h.handlers {
		delete(h.handlers, k)
	}
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
