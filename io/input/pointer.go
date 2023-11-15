// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"image"
	"io"

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
	hitTree   []hitNode
	areas     []areaNode
	cursor    pointer.Cursor
	handlers  map[event.Tag]*pointerHandler
	pointers  []pointerInfo
	transfers []io.ReadCloser // pending data transfers

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

type pointerInfo struct {
	id       pointer.ID
	pressed  bool
	handlers []event.Tag
	// last tracks the last pointer event received,
	// used while processing frame events.
	last pointer.Event

	// entered tracks the tags that contain the pointer.
	entered []event.Tag

	dataSource event.Tag // dragging source tag
	dataTarget event.Tag // dragging target tag
}

type pointerHandler struct {
	area int
	// setup tracks whether the handler has received
	// the pointer.Cancel event that resets its state.
	setup  bool
	active bool
	types  pointer.Kind
	// min and max horizontal/vertical scroll
	scrollRange image.Rectangle

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
	c.state = collectState{}
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
func (c *pointerCollector) newHandler(tag event.Tag) *pointerHandler {
	areaID := c.currentArea()
	c.addHitNode(hitNode{
		area: areaID,
		tag:  tag,
		pass: c.state.pass > 0,
	})
	h := c.q.handlerFor(tag)
	h.area = areaID
	return h
}

func (q *pointerQueue) handlerFor(tag event.Tag) *pointerHandler {
	h, ok := q.handlers[tag]
	if !ok {
		h = &pointerHandler{
			area: -1,
		}
		if q.handlers == nil {
			q.handlers = make(map[event.Tag]*pointerHandler)
		}
		q.handlers[tag] = h
	}
	if !h.active {
		h.types = 0
		h.scrollRange = image.Rectangle{}
		h.sourceMimes = h.sourceMimes[:0]
		h.targetMimes = h.targetMimes[:0]
	}
	h.active = true
	return h
}

func (c *pointerCollector) actionInputOp(act system.Action) {
	areaID := c.currentArea()
	area := &c.q.areas[areaID]
	area.action = act
}

func (q *pointerQueue) grab(evts []taggedEvent, req pointer.GrabCmd) []taggedEvent {
	for _, p := range q.pointers {
		if !p.pressed || p.id != req.ID {
			continue
		}
		// Drop other handlers that lost their grab.
		for i := len(p.handlers) - 1; i >= 0; i-- {
			if tag := p.handlers[i]; tag != req.Tag {
				evts = append(evts, taggedEvent{
					tag:   tag,
					event: pointer.Event{Kind: pointer.Cancel},
				})
				q.dropHandler(tag)
			}
		}
		break
	}
	return evts
}

func (c *pointerCollector) inputOp(tag event.Tag) {
	areaID := c.currentArea()
	area := &c.q.areas[areaID]
	area.semantic.content.tag = tag
	c.newHandler(tag)
}

func (q *pointerQueue) filterTag(tag event.Tag, f pointer.Filter) {
	h := q.handlerFor(tag)
	h.types = h.types | f.Kinds
	h.scrollRange = h.scrollRange.Union(f.ScrollBounds)
}

func (q *pointerQueue) ResetEvent(tag event.Tag) (event.Event, bool) {
	h, ok := q.handlers[tag]
	if !ok || h.setup {
		return nil, false
	}
	h.setup = true
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

func (q *pointerQueue) sourceFilter(tag event.Tag, f transfer.SourceFilter) {
	h := q.handlerFor(tag)
	h.sourceMimes = append(h.sourceMimes, f.Type)
}

func (q *pointerQueue) targetFilter(tag event.Tag, f transfer.TargetFilter) {
	h := q.handlerFor(tag)
	h.targetMimes = append(h.targetMimes, f.Type)
}

func (q *pointerQueue) offerData(evts []taggedEvent, req transfer.OfferCmd) []taggedEvent {
	transferIdx := len(q.transfers)
	q.transfers = append(q.transfers, req.Data)
	for i := range q.pointers {
		p := q.pointers[i]
		if p.dataSource != req.Tag {
			continue
		}
		if p.dataTarget == nil {
			q.pointers[i], evts = q.deliverTransferCancelEvent(p, evts)
			break
		}
		evts = append(evts, taggedEvent{tag: p.dataTarget, event: transfer.DataEvent{
			Type: req.Type,
			Open: func() io.ReadCloser {
				q.transfers[transferIdx] = nil
				return req.Data
			},
		}})
		q.pointers[i], evts = q.deliverTransferCancelEvent(p, evts)
		break
	}
	return evts
}

func (c *pointerCollector) reset() {
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

func (q *pointerQueue) opHit(pos f32.Point) ([]event.Tag, pointer.Cursor) {
	var hits []event.Tag
	cursor := q.hitTest(pos, func(n *hitNode) bool {
		if n.tag != nil {
			if _, exists := q.handlers[n.tag]; exists {
				hits = addHandler(hits, n.tag)
			}
		}
		return true
	})
	return hits, cursor
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
	for _, h := range q.handlers {
		// Reset handler.
		h.area = -1
	}
	q.hitTree = q.hitTree[:0]
	q.areas = q.areas[:0]
	q.semantic.idsAssigned = false
	for k, ids := range q.semantic.contentIDs {
		for i := len(ids) - 1; i >= 0; i-- {
			if !ids[i].used {
				ids = append(ids[:i], ids[i+1:]...)
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
	for _, rc := range q.transfers {
		if rc != nil {
			rc.Close()
		}
	}
	q.transfers = nil
}

func (q *pointerQueue) Frame(evts []taggedEvent) []taggedEvent {
	for k, h := range q.handlers {
		if !h.active {
			q.dropHandler(k)
			delete(q.handlers, k)
			continue
		}
		h.active = false
		if h.area != -1 {
			area := &q.areas[h.area]
			if h.types&(pointer.Press|pointer.Release) != 0 {
				area.semantic.content.gestures |= ClickGesture
			}
			if h.types&pointer.Scroll != 0 {
				area.semantic.content.gestures |= ScrollGesture
			}
			area.semantic.valid = area.semantic.content.gestures != 0
		}
	}
	for i := range q.pointers {
		p := q.pointers[i]
		q.pointers[i], evts = q.deliverEnterLeaveEvents(p, evts, p.last)
	}
	return evts
}

func (q *pointerQueue) dropHandler(tag event.Tag) {
	for i := range q.pointers {
		p := &q.pointers[i]
		for i := len(p.handlers) - 1; i >= 0; i-- {
			if p.handlers[i] == tag {
				p.handlers = append(p.handlers[:i], p.handlers[i+1:]...)
			}
		}
		for i := len(p.entered) - 1; i >= 0; i-- {
			if p.entered[i] == tag {
				p.entered = append(p.entered[:i], p.entered[i+1:]...)
			}
		}
	}
}

// pointerOf returns the pointerInfo index corresponding to the pointer in e.
func (q *pointerQueue) pointerOf(e pointer.Event) int {
	for i, p := range q.pointers {
		if p.id == e.PointerID {
			return i
		}
	}
	q.pointers = append(q.pointers, pointerInfo{id: e.PointerID})
	return len(q.pointers) - 1
}

// Deliver is like Push, but delivers an event to a particular area.
func (q *pointerQueue) Deliver(areaIdx int, e pointer.Event) []taggedEvent {
	var sx, sy = e.Scroll.X, e.Scroll.Y
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
		if n.tag == nil {
			continue
		}
		h := q.handlers[n.tag]
		if h == nil || e.Kind&h.types == 0 {
			continue
		}
		e := e
		if e.Kind == pointer.Scroll {
			if sx == 0 && sy == 0 {
				break
			}
			// Distribute the scroll to the handler based on its ScrollRange.
			sx, e.Scroll.X = setScrollEvent(sx, h.scrollRange.Min.X, h.scrollRange.Max.X)
			sy, e.Scroll.Y = setScrollEvent(sy, h.scrollRange.Min.Y, h.scrollRange.Max.Y)
		}
		e.Position = q.invTransform(h.area, e.Position)
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

func (q *pointerQueue) Push(evts []taggedEvent, e pointer.Event) []taggedEvent {
	if e.Kind == pointer.Cancel {
		for k := range q.handlers {
			evts = append(evts, taggedEvent{
				event: pointer.Event{Kind: pointer.Cancel},
				tag:   k,
			})
		}
		q.pointers = q.pointers[:0]
		for k := range q.handlers {
			q.dropHandler(k)
		}
		return evts
	}
	pidx := q.pointerOf(e)
	p := q.pointers[pidx]

	switch e.Kind {
	case pointer.Press:
		p, evts = q.deliverEnterLeaveEvents(p, evts, e)
		p.pressed = true
		evts = q.deliverEvent(p, evts, e)
	case pointer.Move:
		if p.pressed {
			e.Kind = pointer.Drag
		}
		p, evts = q.deliverEnterLeaveEvents(p, evts, e)
		evts = q.deliverEvent(p, evts, e)
		if p.pressed {
			p, evts = q.deliverDragEvent(p, evts)
		}
	case pointer.Release:
		evts = q.deliverEvent(p, evts, e)
		p.pressed = false
		p, evts = q.deliverEnterLeaveEvents(p, evts, e)
		p, evts = q.deliverDropEvent(p, evts)
	case pointer.Scroll:
		p, evts = q.deliverEnterLeaveEvents(p, evts, e)
		evts = q.deliverEvent(p, evts, e)
	default:
		panic("unsupported pointer event type")
	}

	q.pointers[pidx] = p
	p.last = e

	if !p.pressed && len(p.entered) == 0 {
		// No longer need to track pointer.
		q.pointers = append(q.pointers[:pidx], q.pointers[pidx+1:]...)
	}
	return evts
}

func (q *pointerQueue) deliverEvent(p pointerInfo, evts []taggedEvent, e pointer.Event) []taggedEvent {
	foremost := true
	if p.pressed && len(p.handlers) == 1 {
		e.Priority = pointer.Grabbed
		foremost = false
	}
	var sx, sy = e.Scroll.X, e.Scroll.Y
	for _, k := range p.handlers {
		h := q.handlers[k]
		if e.Kind == pointer.Scroll {
			if sx == 0 && sy == 0 {
				return evts
			}
			// Distribute the scroll to the handler based on its ScrollRange.
			sx, e.Scroll.X = setScrollEvent(sx, h.scrollRange.Min.X, h.scrollRange.Max.X)
			sy, e.Scroll.Y = setScrollEvent(sy, h.scrollRange.Min.Y, h.scrollRange.Max.Y)
		}
		if e.Kind&h.types == 0 {
			continue
		}
		e := e
		if foremost {
			foremost = false
			e.Priority = pointer.Foremost
		}
		e.Position = q.invTransform(h.area, e.Position)
		evts = append(evts, taggedEvent{event: e, tag: k})
	}
	return evts
}

func (q *pointerQueue) deliverEnterLeaveEvents(p pointerInfo, evts []taggedEvent, e pointer.Event) (pointerInfo, []taggedEvent) {
	var hits []event.Tag
	if e.Source != pointer.Mouse && !p.pressed && e.Kind != pointer.Press {
		// Consider non-mouse pointers leaving when they're released.
	} else {
		hits, q.cursor = q.opHit(e.Position)
		if p.pressed {
			// Filter out non-participating handlers,
			// except potential transfer targets when a transfer has been initiated.
			var hitsHaveTarget bool
			if p.dataSource != nil {
				transferSource := q.handlers[p.dataSource]
				for _, hit := range hits {
					if _, ok := firstMimeMatch(transferSource, q.handlers[hit]); ok {
						hitsHaveTarget = true
						break
					}
				}
			}
			for i := len(hits) - 1; i >= 0; i-- {
				if _, found := searchTag(p.handlers, hits[i]); !found && !hitsHaveTarget {
					hits = append(hits[:i], hits[i+1:]...)
				}
			}
		} else {
			p.handlers = hits
		}
	}
	// Deliver Leave events.
	for _, k := range p.entered {
		if _, found := searchTag(hits, k); found {
			continue
		}
		h := q.handlers[k]
		e := e
		e.Kind = pointer.Leave

		if e.Kind&h.types != 0 {
			e.Position = q.invTransform(h.area, e.Position)
			evts = append(evts, taggedEvent{tag: k, event: e})
		}
	}
	// Deliver Enter events.
	for _, k := range hits {
		h := q.handlers[k]
		if _, found := searchTag(p.entered, k); found {
			continue
		}
		e := e
		e.Kind = pointer.Enter

		if e.Kind&h.types != 0 {
			e.Position = q.invTransform(h.area, e.Position)
			evts = append(evts, taggedEvent{tag: k, event: e})
		}
	}
	p.entered = hits
	return p, evts
}

func (q *pointerQueue) deliverDragEvent(p pointerInfo, evts []taggedEvent) (pointerInfo, []taggedEvent) {
	if p.dataSource != nil {
		return p, evts
	}
	// Identify the data source.
	for _, k := range p.entered {
		src := q.handlers[k]
		if len(src.sourceMimes) == 0 {
			continue
		}
		// One data source handler per pointer.
		p.dataSource = k
		// Notify all potential targets.
		for k, tgt := range q.handlers {
			if _, ok := firstMimeMatch(src, tgt); ok {
				evts = append(evts, taggedEvent{tag: k, event: transfer.InitiateEvent{}})
			}
		}
		break
	}
	return p, evts
}

func (q *pointerQueue) deliverDropEvent(p pointerInfo, evts []taggedEvent) (pointerInfo, []taggedEvent) {
	if p.dataSource == nil {
		return p, evts
	}
	// Request data from the source.
	src := q.handlers[p.dataSource]
	for _, k := range p.entered {
		h := q.handlers[k]
		if m, ok := firstMimeMatch(src, h); ok {
			p.dataTarget = k
			evts = append(evts, taggedEvent{tag: p.dataSource, event: transfer.RequestEvent{Type: m}})
			return p, evts
		}
	}
	// No valid target found, abort.
	return q.deliverTransferCancelEvent(p, evts)
}

func (q *pointerQueue) deliverTransferCancelEvent(p pointerInfo, evts []taggedEvent) (pointerInfo, []taggedEvent) {
	evts = append(evts, taggedEvent{tag: p.dataSource, event: transfer.CancelEvent{}})
	// Cancel all potential targets.
	src := q.handlers[p.dataSource]
	for k, h := range q.handlers {
		if _, ok := firstMimeMatch(src, h); ok {
			evts = append(evts, taggedEvent{tag: k, event: transfer.CancelEvent{}})
		}
	}
	p.dataSource = nil
	p.dataTarget = nil
	return p, evts
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
	for _, t := range tags {
		if t == tag {
			return tags
		}
	}
	return append(tags, tag)
}

// firstMimeMatch returns the first type match between src and tgt.
func firstMimeMatch(src, tgt *pointerHandler) (first string, matched bool) {
	for _, m1 := range tgt.targetMimes {
		for _, m2 := range src.sourceMimes {
			if m1 == m2 {
				return m1, true
			}
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

func setScrollEvent(scroll float32, min, max int) (left, scrolled float32) {
	if v := float32(max); scroll > v {
		return scroll - v, v
	}
	if v := float32(min); scroll < v {
		return scroll - v, v
	}
	return 0, scroll
}
