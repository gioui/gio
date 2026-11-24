// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"image"
	"io"
	"maps"
	"slices"

	"gioui.org/f32"
	f32internal "gioui.org/internal/f32"
	"gioui.org/internal/ops"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/io/system"
	"gioui.org/io/transfer"
)

type pointerQueue struct {
	platformDrags bool
	hitTree       []hitNode
	areas         []areaNode

	semantic struct {
		idsAssigned bool
		lastID      SemanticID
		// contentIDs maps semantic content to a list of semantic IDs
		// previously assigned. It is used to maintain stable IDs across
		// frames.
		contentIDs map[semanticContent][]semanticID
	}
}

type hitNode struct {
	next int
	area int

	// For handler nodes.
	tag  event.Tag
	pass bool
}

// pointerState is the input state related to pointer events.
type pointerState struct {
	cursor   pointer.Cursor
	pointers []pointerInfo
	external dropRoute
	outgoing *dragRequest
	local    map[pointer.ID]transferState
}

type pointerInfo struct {
	id pointer.ID
	pointerCapture
	pointerRoute
}

type pointerCapture struct {
	pressed  bool
	handlers []event.Tag
}

type pointerRoute struct {
	last    pointer.Event
	entered []event.Tag
}

type dropRoute struct {
	pointerRoute
	transfer transferState
}

type dragRequest struct {
	tag     event.Tag
	types   []string
	pointer pointer.ID
}

type transferState struct {
	// A nil source means the platform provides the data.
	source  event.Tag
	target  event.Tag
	targets []transferTargetInfo
	types   []string
}

type transferTargetInfo struct {
	tag   event.Tag
	types []string
}

type pointerHandler struct {
	// areaPlusOne is the index into the list of pointerQueue.areas, plus 1.
	areaPlusOne int
	// setup tracks whether the handler has received
	// the pointer.Cancel event that resets its state.
	setup bool
}

// pointerFilter represents the union of a set of pointer filters.
type pointerFilter struct {
	kinds pointer.Kind
	// min and max horizontal/vertical scroll
	scrollX, scrollY pointer.ScrollRange

	sourceMimes []string
	targetMimes []string
}

type areaOp struct {
	kind areaKind
	rect image.Rectangle
}

type areaNode struct {
	trans f32.Affine2D
	area  areaOp

	cursor pointer.Cursor

	// Tree indices, with -1 being the sentinel.
	parent     int
	firstChild int
	lastChild  int
	sibling    int

	semantic struct {
		valid   bool
		id      SemanticID
		content semanticContent
	}
	action system.Action
}

type areaKind uint8

// collectState represents the state for pointerCollector.
type collectState struct {
	t f32.Affine2D
	// nodePlusOne is the current node index, plus one to
	// make the zero value collectState the initial state.
	nodePlusOne int
	pass        int
}

// pointerCollector tracks the state needed to update an pointerQueue
// from pointer ops.
type pointerCollector struct {
	q         *pointerQueue
	state     collectState
	nodeStack []int
}

type semanticContent struct {
	tag      event.Tag
	label    string
	desc     string
	class    semantic.ClassOp
	gestures SemanticGestures
	selected bool
	disabled bool
}

type semanticID struct {
	id   SemanticID
	used bool
}

const (
	areaRect areaKind = iota
	areaEllipse
)

func (c *pointerCollector) resetState() {
	c.state = collectState{
		t: f32.AffineId(),
	}
	c.nodeStack = c.nodeStack[:0]
	// Pop every node except the root.
	if len(c.q.hitTree) > 0 {
		c.state.nodePlusOne = 0 + 1
	}
}

func (c *pointerCollector) setTrans(t f32.Affine2D) {
	c.state.t = t
}

func (c *pointerCollector) clip(op ops.ClipOp) {
	kind := areaRect
	if op.Shape == ops.Ellipse {
		kind = areaEllipse
	}
	c.pushArea(kind, op.Bounds)
}

func (c *pointerCollector) pushArea(kind areaKind, bounds image.Rectangle) {
	parentID := c.currentArea()
	areaID := len(c.q.areas)
	areaOp := areaOp{kind: kind, rect: bounds}
	if parentID != -1 {
		parent := &c.q.areas[parentID]
		if parent.firstChild == -1 {
			parent.firstChild = areaID
		}
		if siblingID := parent.lastChild; siblingID != -1 {
			c.q.areas[siblingID].sibling = areaID
		}
		parent.lastChild = areaID
	}
	an := areaNode{
		trans:      c.state.t,
		area:       areaOp,
		parent:     parentID,
		sibling:    -1,
		firstChild: -1,
		lastChild:  -1,
	}

	c.q.areas = append(c.q.areas, an)
	c.nodeStack = append(c.nodeStack, c.state.nodePlusOne-1)
	c.addHitNode(hitNode{
		area: areaID,
		pass: true,
	})
}

func (c *pointerCollector) popArea() {
	n := len(c.nodeStack)
	c.state.nodePlusOne = c.nodeStack[n-1] + 1
	c.nodeStack = c.nodeStack[:n-1]
}

func (c *pointerCollector) pass() {
	c.state.pass++
}

func (c *pointerCollector) popPass() {
	c.state.pass--
}

func (c *pointerCollector) currentArea() int {
	if i := c.state.nodePlusOne - 1; i != -1 {
		n := c.q.hitTree[i]
		return n.area
	}
	return -1
}

func (c *pointerCollector) currentAreaBounds() image.Rectangle {
	a := c.currentArea()
	if a == -1 {
		panic("no root area")
	}
	return c.q.areas[a].bounds()
}

func (c *pointerCollector) addHitNode(n hitNode) {
	n.next = c.state.nodePlusOne - 1
	c.q.hitTree = append(c.q.hitTree, n)
	c.state.nodePlusOne = len(c.q.hitTree) - 1 + 1
}

// newHandler returns the current handler or a new one for tag.
func (c *pointerCollector) newHandler(tag event.Tag, state *pointerHandler) {
	areaID := c.currentArea()
	c.addHitNode(hitNode{
		area: areaID,
		tag:  tag,
		pass: c.state.pass > 0,
	})
	state.areaPlusOne = areaID + 1
}

func (s *pointerHandler) Reset() {
	s.areaPlusOne = 0
}

func (c *pointerCollector) actionInputOp(act system.Action) {
	areaID := c.currentArea()
	area := &c.q.areas[areaID]
	area.action = act
}

func (q *pointerQueue) grab(state pointerState, req pointer.GrabCmd) (pointerState, []taggedEvent) {
	var evts []taggedEvent
	for _, p := range state.pointers {
		if !p.pressed || p.id != req.ID {
			continue
		}
		// Verify that the grabber is among the handlers.
		found := slices.Contains(p.handlers, req.Tag)
		if !found {
			continue
		}
		// Drop other handlers that lost their grab.
		for _, tag := range slices.Backward(p.handlers) {
			if tag != req.Tag {
				evts = append(evts, taggedEvent{
					tag:   tag,
					event: pointer.Event{Kind: pointer.Cancel},
				})
				state = dropHandler(state, tag)
			}
		}
		break
	}
	return state, evts
}

func (c *pointerCollector) inputOp(tag event.Tag, state *pointerHandler) {
	areaID := c.currentArea()
	area := &c.q.areas[areaID]
	area.semantic.content.tag = tag
	c.newHandler(tag, state)
}

func (p *pointerFilter) Add(f event.Filter) {
	switch f := f.(type) {
	case transfer.SourceFilter:
		if slices.Contains(p.sourceMimes, f.Type) {
			return
		}
		p.sourceMimes = append(p.sourceMimes, f.Type)
	case transfer.TargetFilter:
		if slices.Contains(p.targetMimes, f.Type) {
			return
		}
		p.targetMimes = append(p.targetMimes, f.Type)
	case pointer.Filter:
		p.kinds = p.kinds | f.Kinds
		p.scrollX = p.scrollX.Union(f.ScrollX)
		p.scrollY = p.scrollY.Union(f.ScrollY)
	}
}

func (p *pointerFilter) Matches(e event.Event) bool {
	switch e := e.(type) {
	case pointer.Event:
		return e.Kind&p.kinds == e.Kind
	case transfer.CancelEvent, transfer.InitiateEvent:
		return len(p.sourceMimes) > 0 || len(p.targetMimes) > 0
	case transfer.RequestEvent:
		if slices.Contains(p.sourceMimes, e.Type) {
			return true
		}
	case transfer.DataEvent:
		if slices.Contains(p.targetMimes, e.Type) {
			return true
		}
	}
	return false
}

func (p *pointerFilter) Merge(p2 pointerFilter) {
	p.kinds = p.kinds | p2.kinds
	p.scrollX = p.scrollX.Union(p2.scrollX)
	p.scrollY = p.scrollY.Union(p2.scrollY)
	p.sourceMimes = append(p.sourceMimes, p2.sourceMimes...)
	p.targetMimes = append(p.targetMimes, p2.targetMimes...)
}

// clampScroll splits a scroll distance in the remaining scroll and the
// scroll accepted by the filter.
func (p *pointerFilter) clampScroll(scroll f32.Point) (left, scrolled f32.Point) {
	left.X, scrolled.X = clampSplit(scroll.X, p.scrollX.Min, p.scrollX.Max)
	left.Y, scrolled.Y = clampSplit(scroll.Y, p.scrollY.Min, p.scrollY.Max)
	return
}

func clampSplit(v float32, min, max int) (float32, float32) {
	if m := float32(max); v > m {
		return v - m, m
	}
	if m := float32(min); v < m {
		return v - m, m
	}
	return 0, v
}

func (s *pointerHandler) ResetEvent() (event.Event, bool) {
	if s.setup {
		return nil, false
	}
	s.setup = true
	return pointer.Event{Kind: pointer.Cancel}, true
}

func (c *pointerCollector) semanticLabel(lbl string) {
	areaID := c.currentArea()
	area := &c.q.areas[areaID]
	area.semantic.valid = true
	area.semantic.content.label = lbl
}

func (c *pointerCollector) semanticDesc(desc string) {
	areaID := c.currentArea()
	area := &c.q.areas[areaID]
	area.semantic.valid = true
	area.semantic.content.desc = desc
}

func (c *pointerCollector) semanticClass(class semantic.ClassOp) {
	areaID := c.currentArea()
	area := &c.q.areas[areaID]
	area.semantic.valid = true
	area.semantic.content.class = class
}

func (c *pointerCollector) semanticSelected(selected bool) {
	areaID := c.currentArea()
	area := &c.q.areas[areaID]
	area.semantic.valid = true
	area.semantic.content.selected = selected
}

func (c *pointerCollector) semanticEnabled(enabled bool) {
	areaID := c.currentArea()
	area := &c.q.areas[areaID]
	area.semantic.valid = true
	area.semantic.content.disabled = !enabled
}

func (c *pointerCollector) cursor(cursor pointer.Cursor) {
	areaID := c.currentArea()
	area := &c.q.areas[areaID]
	area.cursor = cursor
}

// transferring reports whether a pointer takes part in a transfer.
func (s pointerState) transferring() bool {
	return len(s.external.transfer.types) > 0 || s.outgoing != nil || len(s.local) != 0
}

func uniqueMimes(mimes []string) []string {
	var unique []string
	for _, mime := range mimes {
		if !slices.Contains(unique, mime) {
			unique = append(unique, mime)
		}
	}
	return unique
}

func (s *pointerState) setLocal(id pointer.ID, drag transferState) {
	if drag.source == nil && s.local[id].source == nil {
		return
	}
	s.local = maps.Clone(s.local)
	if drag.source == nil {
		delete(s.local, id)
		return
	}
	if s.local == nil {
		s.local = make(map[pointer.ID]transferState)
	}
	s.local[id] = drag
}

func (q *pointerQueue) offer(handlers map[event.Tag]*handler, state pointerState, req transfer.OfferCmd) (pointerState, []taggedEvent) {
	var evts []taggedEvent
	for id, drag := range state.local {
		if drag.source != req.Tag {
			continue
		}
		if h, ok := handlers[drag.source]; ok {
			drag.types = h.filter.pointer.sourceMimes
		}
		evts = drag.offer(handlers, transfer.DataEvent{
			Type: req.Type,
			Open: func() io.ReadCloser {
				return req.Data
			},
		}, evts)
		state.setLocal(id, drag)
		break
	}
	return state, evts
}

func (q *pointerQueue) updateTransferTargets(handlers map[event.Tag]*handler, drag *transferState, evts []taggedEvent) []taggedEvent {
	var targets []transferTargetInfo
	for tag, h := range handlers {
		if h.pointer.areaPlusOne == 0 {
			continue
		}
		if matched := mimeIntersection(drag.types, h.filter.pointer.targetMimes); len(matched) > 0 {
			targets = append(targets, transferTargetInfo{tag: tag, types: matched})
		}
	}
	for _, old := range drag.targets {
		if target, ok := findTransferTarget(targets, old.tag); !ok || !slices.Equal(old.types, target.types) {
			evts = append(evts, taggedEvent{tag: old.tag, event: transfer.CancelEvent{}})
		}
	}
	for _, target := range targets {
		if old, ok := findTransferTarget(drag.targets, target.tag); !ok || !slices.Equal(old.types, target.types) {
			evts = append(evts, taggedEvent{tag: target.tag, event: transfer.InitiateEvent{Types: slices.Clone(target.types)}})
		}
	}
	drag.targets = targets
	return evts
}

func findTransferTarget(targets []transferTargetInfo, tag event.Tag) (transferTargetInfo, bool) {
	for _, target := range targets {
		if target.tag == tag {
			return target, true
		}
	}
	return transferTargetInfo{}, false
}

func (q *pointerQueue) initiateExternal(handlers map[event.Tag]*handler, state pointerState, types []string) (pointerState, []taggedEvent) {
	var evts []taggedEvent
	if slices.Equal(state.external.transfer.types, types) {
		return state, nil
	}
	if len(state.external.transfer.types) > 0 {
		state, evts = q.cancelExternal(handlers, state)
	}
	if len(types) > 0 {
		state.external = dropRoute{pointerRoute: pointerRoute{last: pointer.Event{Kind: pointer.Leave}}}
		evts = state.external.transfer.start(handlers, nil, types, state.pressedHandlers(), evts)
	}
	return state, evts
}

func (q *pointerQueue) cancelExternal(handlers map[event.Tag]*handler, state pointerState) (pointerState, []taggedEvent) {
	if len(state.external.transfer.types) == 0 {
		return state, nil
	}
	var evts []taggedEvent
	evts, state.cursor = q.leave(handlers, state.cursor, &state.external, evts)
	evts = state.external.transfer.cancel(handlers, evts)
	state.external = dropRoute{}
	return state, evts
}

func (q *pointerQueue) dropExternal(handlers map[event.Tag]*handler, state pointerState, data transfer.DataEvent) (pointerState, []taggedEvent) {
	route := &state.external
	if len(route.transfer.types) == 0 {
		return state, nil
	}
	var evts []taggedEvent
	if slices.Contains(route.transfer.types, data.Type) && (data.Position != route.last.Position || len(route.entered) == 0) {
		evts, state.cursor = q.move(handlers, state.cursor, &route.pointerRoute, &route.transfer, nil, evts, pointer.Event{
			Kind: pointer.Drag, Source: pointer.Mouse, Position: data.Position,
		})
	}
	var target event.Tag
	if slices.Contains(route.transfer.types, data.Type) {
		target, _ = transferTarget(handlers, []string{data.Type}, route.entered)
	}
	evts, state.cursor = q.leave(handlers, state.cursor, route, evts)
	evts = route.transfer.release(handlers, target, data.Type, evts)
	if target != nil {
		evts = route.transfer.offer(handlers, data, evts)
	}
	state.external = dropRoute{}
	return state, evts
}

// offer delivers data to the drop target and completes the transfer.
func (drag *transferState) offer(handlers map[event.Tag]*handler, data transfer.DataEvent, evts []taggedEvent) []taggedEvent {
	if drag.target != nil {
		evts = append(evts, taggedEvent{tag: drag.target, event: data})
	}
	return drag.cancel(handlers, evts)
}

func (c *pointerCollector) Reset() {
	c.q.reset()
	c.resetState()
	c.ensureRoot()
}

// Ensure implicit root area for semantic descriptions to hang onto.
func (c *pointerCollector) ensureRoot() {
	if len(c.q.areas) > 0 {
		return
	}
	c.pushArea(areaRect, image.Rect(-1e6, -1e6, 1e6, 1e6))
	// Make it semantic to ensure a single semantic root.
	c.q.areas[0].semantic.valid = true
}

func (q *pointerQueue) assignSemIDs() {
	if q.semantic.idsAssigned {
		return
	}
	q.semantic.idsAssigned = true
	for i, a := range q.areas {
		if a.semantic.valid {
			q.areas[i].semantic.id = q.semanticIDFor(a.semantic.content)
		}
	}
}

func (q *pointerQueue) AppendSemantics(nodes []SemanticNode) []SemanticNode {
	q.assignSemIDs()
	nodes = q.appendSemanticChildren(nodes, 0)
	nodes = q.appendSemanticArea(nodes, 0, 0)
	return nodes
}

func (q *pointerQueue) appendSemanticArea(nodes []SemanticNode, parentID SemanticID, nodeIdx int) []SemanticNode {
	areaIdx := nodes[nodeIdx].areaIdx
	a := q.areas[areaIdx]
	childStart := len(nodes)
	nodes = q.appendSemanticChildren(nodes, a.firstChild)
	childEnd := len(nodes)
	for i := childStart; i < childEnd; i++ {
		nodes = q.appendSemanticArea(nodes, a.semantic.id, i)
	}
	n := &nodes[nodeIdx]
	n.ParentID = parentID
	n.Children = nodes[childStart:childEnd]
	return nodes
}

func (q *pointerQueue) appendSemanticChildren(nodes []SemanticNode, areaIdx int) []SemanticNode {
	if areaIdx == -1 {
		return nodes
	}
	a := q.areas[areaIdx]
	if semID := a.semantic.id; semID != 0 {
		cnt := a.semantic.content
		nodes = append(nodes, SemanticNode{
			ID: semID,
			Desc: SemanticDesc{
				Bounds:      a.bounds(),
				Label:       cnt.label,
				Description: cnt.desc,
				Class:       cnt.class,
				Gestures:    cnt.gestures,
				Selected:    cnt.selected,
				Disabled:    cnt.disabled,
			},
			areaIdx: areaIdx,
		})
	} else {
		nodes = q.appendSemanticChildren(nodes, a.firstChild)
	}
	return q.appendSemanticChildren(nodes, a.sibling)
}

func (q *pointerQueue) semanticIDFor(content semanticContent) SemanticID {
	ids := q.semantic.contentIDs[content]
	for i, id := range ids {
		if !id.used {
			ids[i].used = true
			return id.id
		}
	}
	// No prior assigned ID; allocate a new one.
	q.semantic.lastID++
	id := semanticID{id: q.semantic.lastID, used: true}
	if q.semantic.contentIDs == nil {
		q.semantic.contentIDs = make(map[semanticContent][]semanticID)
	}
	q.semantic.contentIDs[content] = append(q.semantic.contentIDs[content], id)
	return id.id
}

func (q *pointerQueue) ActionAt(pos f32.Point) (action system.Action, hasAction bool) {
	q.hitTest(pos, func(n *hitNode) bool {
		area := q.areas[n.area]
		if area.action != 0 {
			action = area.action
			hasAction = true
			return false
		}
		return true
	})
	return action, hasAction
}

func (q *pointerQueue) SemanticAt(pos f32.Point) (semID SemanticID, hasSemID bool) {
	q.assignSemIDs()
	q.hitTest(pos, func(n *hitNode) bool {
		area := q.areas[n.area]
		if area.semantic.id != 0 {
			semID = area.semantic.id
			hasSemID = true
			return false
		}
		return true
	})
	return semID, hasSemID
}

// hitTest searches the hit tree for nodes matching pos. Any node matching pos will
// have the onNode func invoked on it to allow the caller to extract whatever information
// is necessary for further processing. onNode may return false to terminate the walk of
// the hit tree, or true to continue. Providing this algorithm in this generic way
// allows normal event routing and system action event routing to share the same traversal
// logic even though they are interested in different aspects of hit nodes.
func (q *pointerQueue) hitTest(pos f32.Point, onNode func(*hitNode) bool) pointer.Cursor {
	// Track whether we're passing through hits.
	pass := true
	idx := len(q.hitTree) - 1
	cursor := pointer.CursorDefault
	for idx >= 0 {
		n := &q.hitTree[idx]
		hit, c := q.hit(n.area, pos)
		if !hit {
			idx--
			continue
		}
		if cursor == pointer.CursorDefault {
			cursor = c
		}
		pass = pass && n.pass
		if pass {
			idx--
		} else {
			idx = n.next
		}
		if !onNode(n) {
			break
		}
	}
	return cursor
}

func (q *pointerQueue) invTransform(areaIdx int, p f32.Point) f32.Point {
	if areaIdx == -1 {
		return p
	}
	return q.areas[areaIdx].trans.Invert().Transform(p)
}

func (q *pointerQueue) hit(areaIdx int, p f32.Point) (bool, pointer.Cursor) {
	c := pointer.CursorDefault
	for areaIdx != -1 {
		a := &q.areas[areaIdx]
		if c == pointer.CursorDefault {
			c = a.cursor
		}
		p := a.trans.Invert().Transform(p)
		if !a.area.Hit(p) {
			return false, c
		}
		areaIdx = a.parent
	}
	return true, c
}

func (q *pointerQueue) reset() {
	q.hitTree = q.hitTree[:0]
	q.areas = q.areas[:0]
	q.semantic.idsAssigned = false
	for k, ids := range q.semantic.contentIDs {
		for i := len(ids) - 1; i >= 0; i-- {
			if !ids[i].used {
				ids = slices.Delete(ids, i, i+1)
			} else {
				ids[i].used = false
			}
		}
		if len(ids) > 0 {
			q.semantic.contentIDs[k] = ids
		} else {
			delete(q.semantic.contentIDs, k)
		}
	}
}

func (q *pointerQueue) Frame(handlers map[event.Tag]*handler, state pointerState) (pointerState, []taggedEvent) {
	for _, h := range handlers {
		if h.pointer.areaPlusOne != 0 {
			area := &q.areas[h.pointer.areaPlusOne-1]
			if h.filter.pointer.kinds&(pointer.Press|pointer.Release) != 0 {
				area.semantic.content.gestures |= ClickGesture
			}
			if h.filter.pointer.kinds&pointer.Scroll != 0 {
				area.semantic.content.gestures |= ScrollGesture
			}
			area.semantic.valid = area.semantic.content.gestures != 0
		}
	}
	var evts []taggedEvent
	if len(state.external.transfer.types) > 0 {
		evts = q.updateTransferTargets(handlers, &state.external.transfer, evts)
		evts, state.cursor, _ = q.deliverEnterLeaveEvents(handlers, state.cursor, &state.external.pointerRoute, &state.external.transfer, nil, evts, state.external.last)
	}
	for i, p := range state.pointers {
		changed := false
		drag := state.local[p.id]
		evts, state.cursor, changed = q.deliverEnterLeaveEvents(handlers, state.cursor, &p.pointerRoute, &drag, &p.pointerCapture, evts, p.last)
		if changed {
			state.setLocal(p.id, drag)
			state.pointers = slices.Clone(state.pointers)
			state.pointers[i] = p
		}
	}
	return state, evts
}

func dropHandler(state pointerState, tag event.Tag) pointerState {
	pointers := state.pointers
	state.pointers = nil
	for _, p := range pointers {
		handlers := p.handlers
		p.handlers = nil
		for _, h := range handlers {
			if h != tag {
				p.handlers = append(p.handlers, h)
			}
		}
		entered := p.entered
		p.entered = nil
		for _, h := range entered {
			if h != tag {
				p.entered = append(p.entered, h)
			}
		}
		state.pointers = append(state.pointers, p)
	}
	if len(state.external.transfer.types) > 0 {
		state.external.entered = slices.DeleteFunc(slices.Clone(state.external.entered), func(h event.Tag) bool { return h == tag })
		state.external.transfer.targets = slices.DeleteFunc(slices.Clone(state.external.transfer.targets), func(target transferTargetInfo) bool { return target.tag == tag })
	}
	return state
}

// pointerOf returns the pointerInfo index corresponding to the pointer in e.
func (s pointerState) pointerOf(e pointer.Event) (pointerState, int) {
	for i, p := range s.pointers {
		if p.id == e.PointerID {
			return s, i
		}
	}
	n := len(s.pointers)
	s.pointers = append(s.pointers[:n:n], pointerInfo{id: e.PointerID})
	return s, len(s.pointers) - 1
}

// Deliver is like Push, but delivers an event to a particular area.
func (q *pointerQueue) Deliver(handlers map[event.Tag]*handler, areaIdx int, e pointer.Event) []taggedEvent {
	scroll := e.Scroll
	idx := len(q.hitTree) - 1
	// Locate first potential receiver.
	for idx != -1 {
		n := &q.hitTree[idx]
		if n.area == areaIdx {
			break
		}
		idx--
	}
	var evts []taggedEvent
	for idx != -1 {
		n := &q.hitTree[idx]
		idx = n.next
		h, ok := handlers[n.tag]
		if !ok || !h.filter.pointer.Matches(e) {
			continue
		}
		e := e
		if e.Kind == pointer.Scroll {
			if scroll == (f32.Point{}) {
				break
			}
			scroll, e.Scroll = h.filter.pointer.clampScroll(scroll)
		}
		e.Position = q.invTransform(h.pointer.areaPlusOne-1, e.Position)
		evts = append(evts, taggedEvent{tag: n.tag, event: e})
		if e.Kind != pointer.Scroll {
			break
		}
	}
	return evts
}

// SemanticArea returns the sematic content for area, and its parent area.
func (q *pointerQueue) SemanticArea(areaIdx int) (semanticContent, int) {
	for areaIdx != -1 {
		a := &q.areas[areaIdx]
		areaIdx = a.parent
		if !a.semantic.valid {
			continue
		}
		return a.semantic.content, areaIdx
	}
	return semanticContent{}, -1
}

func (q *pointerQueue) Push(handlers map[event.Tag]*handler, state pointerState, e pointer.Event) (pointerState, []taggedEvent) {
	var evts []taggedEvent
	if e.Kind == pointer.Drag && len(state.external.transfer.types) > 0 {
		evts, state.cursor = q.move(handlers, state.cursor, &state.external.pointerRoute, &state.external.transfer, nil, evts, e)
		return state, evts
	}
	if e.Kind == pointer.Drag {
		return state, nil
	}
	if e.Kind == pointer.Cancel {
		if len(state.external.transfer.types) > 0 {
			state, evts = q.cancelExternal(handlers, state)
		}
		for _, drag := range state.local {
			evts = drag.cancel(handlers, evts)
		}
		state.local = nil
		if req := state.outgoing; req != nil {
			evts = append(evts, taggedEvent{tag: req.tag, event: transfer.CancelEvent{}})
			state.outgoing = nil
		}
		for k := range handlers {
			evts = append(evts, taggedEvent{
				event: pointer.Event{Kind: pointer.Cancel},
				tag:   k,
			})
		}
		state.pointers = nil
		return state, evts
	}
	if e.Kind == pointer.Scroll {
		// Scroll events are not bound to a pointer; see pointer.Event.PointerID.
		return state, q.deliverScrollEvent(handlers, evts, e)
	}
	state, pidx := state.pointerOf(e)
	p := state.pointers[pidx]
	drag := state.local[p.id]

	switch e.Kind {
	case pointer.Press:
		evts, state.cursor, _ = q.deliverEnterLeaveEvents(handlers, state.cursor, &p.pointerRoute, &drag, &p.pointerCapture, evts, e)
		p.pressed = true
		evts = q.deliverEvent(handlers, p.handlers, &p.pointerCapture, evts, e)
	case pointer.Move:
		if p.pressed {
			e.Kind = pointer.Drag
			if drag.source == nil && state.outgoing == nil {
				for _, tag := range p.entered {
					h := handlers[tag]
					if h == nil || len(h.filter.pointer.sourceMimes) == 0 {
						continue
					}
					types := h.filter.pointer.sourceMimes
					if q.platformDrags && e.Source == pointer.Mouse && e.Buttons == pointer.ButtonPrimary {
						unique := uniqueMimes(types)
						state.outgoing = &dragRequest{tag: tag, types: unique, pointer: p.id}
						for _, mime := range unique {
							evts = append(evts, taggedEvent{tag: tag, event: transfer.RequestEvent{Type: mime}})
						}
					} else {
						evts = drag.start(handlers, tag, types, state.pressedHandlers(), evts)
					}
					break
				}
			}
		}
		evts, state.cursor = q.move(handlers, state.cursor, &p.pointerRoute, &drag, &p.pointerCapture, evts, e)
	case pointer.Leave:
		evts, state.cursor, _ = q.deliverEnterLeaveEvents(handlers, state.cursor, &p.pointerRoute, &drag, &p.pointerCapture, evts, e)
	case pointer.Release:
		evts = q.deliverEvent(handlers, p.handlers, &p.pointerCapture, evts, e)
		p.pressed = false
		evts, state.cursor, _ = q.deliverEnterLeaveEvents(handlers, state.cursor, &p.pointerRoute, &drag, &p.pointerCapture, evts, e)
		if drag.source != nil {
			target, mime := transferTarget(handlers, drag.types, p.entered)
			evts = drag.release(handlers, target, mime, evts)
		}
		if req := state.outgoing; req != nil && req.pointer == p.id {
			evts = append(evts, taggedEvent{tag: req.tag, event: transfer.CancelEvent{}})
			state.outgoing = nil
		}
	default:
		panic("unsupported pointer event type")
	}

	p.last = e
	state.setLocal(p.id, drag)

	if !p.pressed && len(p.entered) == 0 {
		// No longer need to track pointer.
		state.pointers = slices.Concat(state.pointers[:pidx:pidx], state.pointers[pidx+1:])
	} else {
		state.pointers = slices.Clone(state.pointers)
		state.pointers[pidx] = p
	}
	return state, evts
}

// deliverScrollEvent delivers scroll events to the handlers hit by the event coordinate.
func (q *pointerQueue) deliverScrollEvent(handlers map[event.Tag]*handler, evts []taggedEvent, e pointer.Event) []taggedEvent {
	var hits []event.Tag
	q.hitTest(e.Position, func(n *hitNode) bool {
		if _, ok := handlers[n.tag]; ok {
			hits = addHandler(hits, n.tag)
		}
		return true
	})
	return q.deliverEvent(handlers, hits, nil, evts, e)
}

func (q *pointerQueue) move(handlers map[event.Tag]*handler, cursor pointer.Cursor, route *pointerRoute, drag *transferState, capture *pointerCapture, evts []taggedEvent, e pointer.Event) ([]taggedEvent, pointer.Cursor) {
	evts, cursor, _ = q.deliverEnterLeaveEvents(handlers, cursor, route, drag, capture, evts, e)
	route.last = e
	targets := route.entered
	if capture != nil {
		targets = capture.handlers
	}
	if capture != nil && e.Kind == pointer.Drag && len(drag.types) > 0 {
		// Drop targets receive drag events even when the source has grabbed the pointer.
		targets = slices.Clone(targets)
		for _, k := range route.entered {
			targets = addHandler(targets, k)
		}
	}
	return q.deliverEvent(handlers, targets, capture, evts, e), cursor
}

func (q *pointerQueue) deliverEvent(handlers map[event.Tag]*handler, targets []event.Tag, capture *pointerCapture, evts []taggedEvent, e pointer.Event) []taggedEvent {
	scroll := e.Scroll
	for _, k := range targets {
		h, ok := handlers[k]
		if !ok {
			continue
		}
		f := h.filter.pointer
		if !f.Matches(e) {
			continue
		}
		if e.Kind == pointer.Scroll {
			if scroll == (f32.Point{}) {
				return evts
			}
			scroll, e.Scroll = f.clampScroll(scroll)
		}
		e := e
		if capture != nil && capture.pressed && len(capture.handlers) == 1 && k == capture.handlers[0] {
			e.Priority = pointer.Grabbed
		}
		e.Position = q.invTransform(h.pointer.areaPlusOne-1, e.Position)
		evts = append(evts, taggedEvent{event: e, tag: k})
	}
	return evts
}

func (q *pointerQueue) leave(handlers map[event.Tag]*handler, cursor pointer.Cursor, route *dropRoute, evts []taggedEvent) ([]taggedEvent, pointer.Cursor) {
	e := route.last
	e.Kind = pointer.Leave
	evts, cursor, _ = q.deliverEnterLeaveEvents(handlers, cursor, &route.pointerRoute, &route.transfer, nil, evts, e)
	return evts, cursor
}

func (q *pointerQueue) deliverEnterLeaveEvents(handlers map[event.Tag]*handler, cursor pointer.Cursor, route *pointerRoute, drag *transferState, capture *pointerCapture, evts []taggedEvent, e pointer.Event) ([]taggedEvent, pointer.Cursor, bool) {
	changed := false
	if drag.source != nil {
		var types []string
		if h, ok := handlers[drag.source]; ok {
			types = h.filter.pointer.sourceMimes
		}
		if !slices.Equal(drag.types, types) {
			drag.types = slices.Clone(types)
			changed = true
		}
	}
	var hits []event.Tag
	if e.Kind == pointer.Leave || e.Source != pointer.Mouse && (capture == nil || !capture.pressed) && e.Kind != pointer.Press {
		// Consider non-mouse pointers leaving when they're released.
	} else {
		cursor = q.hitTest(e.Position, func(n *hitNode) bool {
			h, ok := handlers[n.tag]
			if !ok {
				return true
			}
			add := true
			_, matches := firstMimeMatch(drag.types, &h.filter.pointer)
			pressed := capture != nil && capture.pressed
			if matches || capture != nil && (!pressed || slices.Contains(capture.handlers, n.tag)) {
				add = true
			} else {
				add = false
			}
			if add {
				hits = addHandler(hits, n.tag)
			}
			return true
		})
		if capture != nil && !capture.pressed {
			changed = true
			capture.handlers = hits
		}
	}
	// Deliver Leave events.
	for _, k := range route.entered {
		if _, found := searchTag(hits, k); found {
			continue
		}
		h, ok := handlers[k]
		if !ok {
			continue
		}
		changed = true
		e := e
		e.Kind = pointer.Leave

		if h.filter.pointer.Matches(e) {
			e.Position = q.invTransform(h.pointer.areaPlusOne-1, e.Position)
			evts = append(evts, taggedEvent{tag: k, event: e})
		}
	}
	// Deliver Enter events.
	for _, k := range hits {
		if _, found := searchTag(route.entered, k); found {
			continue
		}
		h, ok := handlers[k]
		if !ok {
			continue
		}
		changed = true
		e := e
		e.Kind = pointer.Enter

		if h.filter.pointer.Matches(e) {
			e.Position = q.invTransform(h.pointer.areaPlusOne-1, e.Position)
			evts = append(evts, taggedEvent{tag: k, event: e})
		}
	}
	route.entered = hits
	return evts, cursor, changed
}

// start initiates a transfer of types from source. Handlers that are
// neither potential targets nor captured by a pressed pointer don't
// take part in the transfer and receive a pointer.Cancel.
func (drag *transferState) start(handlers map[event.Tag]*handler, source event.Tag, types []string, captured []event.Tag, evts []taggedEvent) []taggedEvent {
	*drag = transferState{source: source, types: slices.Clone(types)}
	for k, h := range handlers {
		if h.pointer.areaPlusOne == 0 {
			continue
		}
		if matched := mimeIntersection(types, h.filter.pointer.targetMimes); len(matched) > 0 {
			drag.targets = append(drag.targets, transferTargetInfo{tag: k, types: matched})
			evts = append(evts, taggedEvent{tag: k, event: transfer.InitiateEvent{Types: slices.Clone(matched)}})
		} else if !slices.Contains(captured, k) {
			evts = append(evts, taggedEvent{tag: k, event: pointer.Event{Kind: pointer.Cancel}})
		}
	}
	return evts
}

// pressedHandlers returns the handlers captured by pressed pointers.
func (s pointerState) pressedHandlers() []event.Tag {
	var captured []event.Tag
	for _, p := range s.pointers {
		if p.pressed {
			captured = append(captured, p.handlers...)
		}
	}
	return captured
}

// release selects the drop target and requests the data from the source.
// The transfer is canceled if there is no target.
func (drag *transferState) release(handlers map[event.Tag]*handler, target event.Tag, mime string, evts []taggedEvent) []taggedEvent {
	drag.target = target
	if target == nil {
		return drag.cancel(handlers, evts)
	}
	if drag.source != nil {
		evts = append(evts, taggedEvent{tag: drag.source, event: transfer.RequestEvent{Type: mime}})
	}
	return evts
}

func transferTarget(handlers map[event.Tag]*handler, types []string, targets []event.Tag) (event.Tag, string) {
	for _, k := range targets {
		h, ok := handlers[k]
		if !ok {
			continue
		}
		if mime, ok := firstMimeMatch(types, &h.filter.pointer); ok {
			return k, mime
		}
	}
	return nil, ""
}

// acceptsTransferAt reports whether a handler at position accepts mime.
func (q *pointerQueue) acceptsTransferAt(handlers map[event.Tag]*handler, mime string, position f32.Point) bool {
	accept := false
	q.hitTest(position, func(n *hitNode) bool {
		h, ok := handlers[n.tag]
		accept = ok && slices.Contains(h.filter.pointer.targetMimes, mime)
		return !accept
	})
	return accept
}

// cancel ends the transfer and notifies the source and all potential targets.
func (drag *transferState) cancel(handlers map[event.Tag]*handler, evts []taggedEvent) []taggedEvent {
	if drag.source != nil {
		evts = append(evts, taggedEvent{tag: drag.source, event: transfer.CancelEvent{}})
	}
	for _, k := range drag.targets {
		evts = append(evts, taggedEvent{tag: k.tag, event: transfer.CancelEvent{}})
	}
	*drag = transferState{}
	return evts
}

// ClipFor clips r to the parents of area.
func (q *pointerQueue) ClipFor(area int, r image.Rectangle) image.Rectangle {
	a := &q.areas[area]
	parent := a.parent
	for parent != -1 {
		a := &q.areas[parent]
		r = r.Intersect(a.bounds())
		parent = a.parent
	}
	return r
}

func searchTag(tags []event.Tag, tag event.Tag) (int, bool) {
	for i, t := range tags {
		if t == tag {
			return i, true
		}
	}
	return 0, false
}

// addHandler adds tag to the slice if not present.
func addHandler(tags []event.Tag, tag event.Tag) []event.Tag {
	if slices.Contains(tags, tag) {
		return tags
	}
	return append(tags, tag)
}

// firstMimeMatch returns the first type match between src and tgt.
func firstMimeMatch(src []string, tgt *pointerFilter) (first string, matched bool) {
	for _, m1 := range tgt.targetMimes {
		if slices.Contains(src, m1) {
			return m1, true
		}
	}
	return "", false
}

func (op *areaOp) Hit(pos f32.Point) bool {
	pos = pos.Sub(f32internal.FPt(op.rect.Min))
	size := f32internal.FPt(op.rect.Size())
	switch op.kind {
	case areaRect:
		return 0 <= pos.X && pos.X < size.X &&
			0 <= pos.Y && pos.Y < size.Y
	case areaEllipse:
		rx := size.X / 2
		ry := size.Y / 2
		xh := pos.X - rx
		yk := pos.Y - ry
		// The ellipse function works in all cases because
		// 0/0 is not <= 1.
		return (xh*xh)/(rx*rx)+(yk*yk)/(ry*ry) <= 1
	default:
		panic("invalid area kind")
	}
}

func (a *areaNode) bounds() image.Rectangle {
	return f32internal.Rectangle{
		Min: a.trans.Transform(f32internal.FPt(a.area.rect.Min)),
		Max: a.trans.Transform(f32internal.FPt(a.area.rect.Max)),
	}.Round()
}
