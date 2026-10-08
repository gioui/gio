// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"sort"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
)

// ─── Layoutable ───────────────────────────────────────────────────────────────

// Layoutable is a positionable, resizable widget container that exposes
// its top-left corner and size and can hold child widgets.
type Layoutable struct {
	Tag   event.Tag
	X, Y float32
	W, H float32
	Child layout.Widget
}

// SetRect sets position and size.
func (lb *Layoutable) SetRect(x, y, w, h float32) {
	lb.X, lb.Y = x, y
	lb.W, lb.H = w, h
}

// Rect returns current top-left and size.
func (lb *Layoutable) Rect() (x, y, w, h float32) {
	return lb.X, lb.Y, lb.W, lb.H
}

// Layout lays out a Layoutable and its child at its current rectangle.
func (lb *Layoutable) Layout(gtx layout.Context) layout.Dimensions {
	if lb.Child == nil {
		return layout.Dimensions{}
	}
	macro := op.Record(gtx.Ops)
	op.Offset(image.Point{X: int(lb.X), Y: int(lb.Y)}).Add(gtx.Ops)
	dims := lb.Child(gtx)
	op.Defer(gtx.Ops, macro.Stop())
	return dims
}

// MakeLayoutable creates a Layoutable with the given rectangle and child.
func MakeLayoutable(tag event.Tag, x, y, w, h float32, child layout.Widget) *Layoutable {
	return &Layoutable{
		Tag:   tag,
		X:     x,
		Y:     y,
		W:     w,
		H:     h,
		Child: child,
	}
}

// ─── DragObject ───────────────────────────────────────────────────────────────

// DragObject is the property map for one draggable object.
// It carries all zone-type fields so the editor can use it directly.
type DragObject struct {
	ID       event.Tag
	ParentID event.Tag

	// Geometry
	OrigX, OrigY float32
	Width, Height float32

	// Constraints (0 = no constraint)
	MinWidth, MinHeight float32
	MaxWidth, MaxHeight float32

	// Zone identity
	Label    string // human-readable name (e.g. "Area 3")
	AreaType string // "generic", "text", "img", "clr"
	ServerID int    // backend persistence ID

	// Rendering order
	ZIndex int // depth / z-order (higher = rendered on top)

	// Display
	Visible   bool
	FillColor string  // background fill color (hex or name)
	BgOpacity float32 // background opacity 0..1
	Opacity   float32 // overall object opacity 0..1
	TextColor string  // text foreground color (hex or name)
	TextAlign string  // "left", "center", "right", "justify"

	// Text zone properties
	TextContent  string // plain text or template variables
	TextTransform string
	DefaultFont  string // zone-level default font path

	// Extensible attributes (fontBold, fontItalic, fontPath, fontSize, etc.)
	AreaAttribs map[string]interface{}

	// Optional child widget
	Child layout.Widget
}

// ParentBounds returns parent bounds or nil if parent not set.
func (o *DragObject) ParentBounds(reg *DragRegistry) (parent *DragObject, inParent bool) {
	if o.ParentID == nil || reg == nil {
		return nil, false
	}
	p := reg.GetObject(o.ParentID)
	if p == nil {
		return nil, false
	}
	return p, true
}

// ClampToParent constrains the object position to parent bounds.
func (o *DragObject) ClampToParent(reg *DragRegistry) {
	p, ok := o.ParentBounds(reg)
	if !ok {
		return
	}
	if o.OrigX < p.OrigX {
		o.OrigX = p.OrigX
	}
	if o.OrigY < p.OrigY {
		o.OrigY = p.OrigY
	}
	if o.OrigX+o.Width > p.OrigX+p.Width {
		o.OrigX = p.OrigX + p.Width - o.Width
	}
	if o.OrigY+o.Height > p.OrigY+p.Height {
		o.OrigY = p.OrigY + p.Height - o.Height
	}
}

// ClampToCanvas constrains the object position so it stays within the given canvas rectangle.
// If canvasW or canvasH is 0, clamping is skipped on that axis.
func (o *DragObject) ClampToCanvas(canvasW, canvasH float32) {
	if canvasW > 0 {
		if o.OrigX < 0 {
			o.OrigX = 0
		}
		if o.OrigX+o.Width > canvasW {
			o.OrigX = canvasW - o.Width
		}
	}
	if canvasH > 0 {
		if o.OrigY < 0 {
			o.OrigY = 0
		}
		if o.OrigY+o.Height > canvasH {
			o.OrigY = canvasH - o.Height
		}
	}
}

// ClampSize enforces MinWidth/MinHeight/MaxWidth/MaxHeight constraints.
func (o *DragObject) ClampSize() {
	if o.MinWidth > 0 && o.Width < o.MinWidth {
		o.Width = o.MinWidth
	}
	if o.MinHeight > 0 && o.Height < o.MinHeight {
		o.Height = o.MinHeight
	}
	if o.MaxWidth > 0 && o.Width > o.MaxWidth {
		o.Width = o.MaxWidth
	}
	if o.MaxHeight > 0 && o.Height > o.MaxHeight {
		o.Height = o.MaxHeight
	}
}

// Bounds returns the object rectangle.
func (o *DragObject) Bounds() Rectangle {
	return Rectangle{
		Min: f32.Point{X: o.OrigX, Y: o.OrigY},
		Max: f32.Point{X: o.OrigX + o.Width, Y: o.OrigY + o.Height},
	}
}

// Contains reports whether p is inside the object rectangle.
func (o *DragObject) Contains(p f32.Point) bool {
	return p.X >= o.OrigX && p.X <= o.OrigX+o.Width &&
		p.Y >= o.OrigY && p.Y <= o.OrigY+o.Height
}

// ─── DragRegistry ─────────────────────────────────────────────────────────────

// DragRegistry is a map-backed, indexed store of draggable objects.
// It supports dynamic add/remove, access by ID, and depth-sorted iteration.
type DragRegistry struct {
	Objects map[event.Tag]*DragObject
	Index   []event.Tag // order-preserving index for iteration
	Tag     event.Tag   // shared pointer tag
}

// NewDragRegistry creates a new DragRegistry.
func NewDragRegistry(tag event.Tag) *DragRegistry {
	return &DragRegistry{
		Objects: make(map[event.Tag]*DragObject),
		Tag:     tag,
	}
}

// AddObject inserts or updates an object.
func (dr *DragRegistry) AddObject(id event.Tag, obj *DragObject) {
	if obj == nil {
		return
	}
	if _, exists := dr.Objects[id]; !exists {
		dr.Index = append(dr.Index, id)
	}
	dr.Objects[id] = obj
	obj.ClampToParent(dr)
}

// RemoveObject removes an object by id.
func (dr *DragRegistry) RemoveObject(id event.Tag) {
	delete(dr.Objects, id)
	newIndex := make([]event.Tag, 0, len(dr.Index))
	for _, idx := range dr.Index {
		if idx != id {
			newIndex = append(newIndex, idx)
		}
	}
	dr.Index = newIndex
}

// GetObject returns an object by id or nil.
func (dr *DragRegistry) GetObject(id event.Tag) *DragObject {
	if dr.Objects == nil {
		return nil
	}
	return dr.Objects[id]
}

// SetParent sets the parent of an object within the same registry.
func (dr *DragRegistry) SetParent(id, parentID event.Tag) {
	if obj, ok := dr.Objects[id]; ok {
		obj.ParentID = parentID
	}
}

// ForEachObject calls fn for every object in index order.
func (dr *DragRegistry) ForEachObject(fn func(id event.Tag, obj *DragObject)) {
	for _, id := range dr.Index {
		if obj, ok := dr.Objects[id]; ok {
			fn(id, obj)
		}
	}
}

// ForEachObjectByDepth calls fn for every object sorted ascending by ZIndex (depth).
// Objects with the same ZIndex keep their index order (stable sort).
func (dr *DragRegistry) ForEachObjectByDepth(fn func(id event.Tag, obj *DragObject)) {
	if len(dr.Index) <= 1 {
		dr.ForEachObject(fn)
		return
	}
	type depthEntry struct {
		id  event.Tag
		obj *DragObject
	}
	entries := make([]depthEntry, 0, len(dr.Index))
	for _, id := range dr.Index {
		if obj, ok := dr.Objects[id]; ok {
			entries = append(entries, depthEntry{id: id, obj: obj})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].obj.ZIndex < entries[j].obj.ZIndex
	})
	for _, e := range entries {
		fn(e.id, e.obj)
	}
}

// ForEachObjectByDepthReverse calls fn for every object sorted descending by ZIndex.
// Highest ZIndex first — use this for hit-testing (topmost object wins).
func (dr *DragRegistry) ForEachObjectByDepthReverse(fn func(id event.Tag, obj *DragObject)) {
	if len(dr.Index) <= 1 {
		dr.ForEachObject(fn)
		return
	}
	type depthEntry struct {
		id  event.Tag
		obj *DragObject
	}
	entries := make([]depthEntry, 0, len(dr.Index))
	for _, id := range dr.Index {
		if obj, ok := dr.Objects[id]; ok {
			entries = append(entries, depthEntry{id: id, obj: obj})
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].obj.ZIndex > entries[j].obj.ZIndex
	})
	for _, e := range entries {
		fn(e.id, e.obj)
	}
}

// HitTest returns the topmost (highest ZIndex) visible object containing point p.
// Returns nil if no object contains p.
func (dr *DragRegistry) HitTest(p f32.Point) *DragObject {
	var hit *DragObject
	dr.ForEachObjectByDepthReverse(func(id event.Tag, obj *DragObject) {
		if hit != nil {
			return // already found a closer match
		}
		if !obj.Visible {
			return
		}
		if obj.Contains(p) {
			hit = obj
		}
	})
	return hit
}

// UpdateObjectPosition updates position and clamps to parent + canvas.
func (dr *DragRegistry) UpdateObjectPosition(id event.Tag, x, y float32) {
	obj := dr.GetObject(id)
	if obj == nil {
		return
	}
	obj.OrigX = x
	obj.OrigY = y
	obj.ClampToParent(dr)
}

// UpdateObjectSize updates size with min/max clamping.
func (dr *DragRegistry) UpdateObjectSize(id event.Tag, w, h float32) {
	obj := dr.GetObject(id)
	if obj == nil {
		return
	}
	obj.Width = w
	obj.Height = h
	obj.ClampSize()
	obj.ClampToParent(dr)
}

// UpdateObjectVisibility updates visibility.
func (dr *DragRegistry) UpdateObjectVisibility(id event.Tag, visible bool) {
	if obj := dr.GetObject(id); obj != nil {
		obj.Visible = visible
	}
}

// UpdateObjectDisplay updates display properties.
func (dr *DragRegistry) UpdateObjectDisplay(id event.Tag, fill, stroke string, strokeWidth, opacity float32, zIndex int) {
	obj := dr.GetObject(id)
	if obj == nil {
		return
	}
	obj.FillColor = fill
	obj.Opacity = opacity
	obj.ZIndex = zIndex
}

// ─── DragState ────────────────────────────────────────────────────────────────

// DragState is the state of a positionable widget candidate during direct manipulation.
type DragState struct {
	Tag              event.Tag
	OriginX, OriginY float32
	Width, Height    float32
	Layoutable       *Layoutable
	PressedAt        f32.Point
	PressedAtOriginX float32
	PressedAtOriginY float32
	Active           bool
	DragDelta        f32.Point
	ResizeState      ResizeState
	LastPointer      f32.Point
	// Drag threshold: pixels of movement before drag actually starts.
	// Prevents accidental drags from shaky clicks.
	DragStarted bool
}

// ResizeState describes which resize handle is active, if any.
type ResizeState int

const (
	ResizeNone ResizeState = iota
	ResizeLeft
	ResizeRight
	ResizeTop
	ResizeBottom
	ResizeTopLeft
	ResizeTopRight
	ResizeBottomLeft
	ResizeBottomRight
)

// HandleSizeForEdge returns the hit-test size for a given resize handle.
// Corner handles are slightly larger than edge handles for easier targeting.
func HandleSizeForEdge(state ResizeState, base int) int {
	switch state {
	case ResizeTopLeft, ResizeTopRight, ResizeBottomLeft, ResizeBottomRight:
		return base + 4
	default:
		return base
	}
}

// Rect returns the current live rectangle for a candidate.
func (ws *DragState) Rect() (x, y, w, h float32) {
	return ws.OriginX, ws.OriginY, ws.Width, ws.Height
}

func (ws *DragState) OriginPoint() f32.Point {
	return f32.Point{X: ws.OriginX, Y: ws.OriginY}
}

func (ws *DragState) MaxPoint() f32.Point {
	return f32.Point{X: ws.OriginX + ws.Width, Y: ws.OriginY + ws.Height}
}

// ResetCandidateGeometry resets a candidate to a clean initial rectangle.
func (ws *DragState) ResetCandidateGeometry(x, y, w, h float32) {
	ws.OriginX, ws.OriginY = x, y
	ws.Width, ws.Height = w, h
	ws.PressedAt = f32.Point{}
	ws.PressedAtOriginX = x
	ws.PressedAtOriginY = y
	ws.DragDelta = f32.Point{}
	ws.LastPointer = f32.Point{}
	ws.ResizeState = ResizeNone
	ws.Active = false
	ws.DragStarted = false
}

// SyncCandidateGeometry writes the candidate geometry back into the linked Layoutable.
func (ws *DragState) SyncCandidateGeometry() {
	if ws.Layoutable == nil {
		return
	}
	ws.Layoutable.X, ws.Layoutable.Y = ws.OriginX, ws.OriginY
	ws.Layoutable.W, ws.Layoutable.H = ws.Width, ws.Height
}

// ─── DragAndResize ────────────────────────────────────────────────────────────

// DragAndResize keeps a slice of DragState candidates you select and manipulate
// before committing them into final widgets/zones.
type DragAndResize struct {
	Tag        event.Tag
	Candidates []*DragState
	Selected   *DragState
	PtrFilter  pointer.Filter
	// HandleSize is the base size in pixels of the resize/shrink handles.
	HandleSize int
	// DragThreshold is the minimum pixel movement before a drag starts.
	DragThreshold float32
	// CanvasWidth/CanvasHeight constrain drag bounds (0 = no constraint).
	CanvasWidth, CanvasHeight float32
	// MinSize is the minimum width/height for all candidates.
	MinSize float32
	// MaxSize is the maximum width/height for all candidates (0 = no max).
	MaxSize float32
}

// NewDragAndResize creates a DragAndResize with the given tag.
func NewDragAndResize(tag event.Tag) *DragAndResize {
	return &DragAndResize{
		Tag:           tag,
		HandleSize:    10,
		DragThreshold: 4,
		MinSize:       10,
	}
}

// SetHandleSize sets the handle size used by this DragAndResize.
func (dr *DragAndResize) SetHandleSize(size int) {
	if size < 4 {
		size = 4
	}
	dr.HandleSize = size
}

// SetCanvasBounds sets the canvas dimensions for drag clamping.
func (dr *DragAndResize) SetCanvasBounds(w, h float32) {
	dr.CanvasWidth = w
	dr.CanvasHeight = h
}

// SetSizeConstraints sets min/max size for all candidates.
func (dr *DragAndResize) SetSizeConstraints(minW, maxW float32) {
	dr.MinSize = minW
	dr.MaxSize = maxW
}

// AddCandidate attaches or updates a candidate and syncs it with a Layoutable.
func (dr *DragAndResize) AddCandidate(tag event.Tag, layoutable *Layoutable) *DragState {
	ws := dr.selectedOrNew(tag)
	if layoutable != nil {
		ws.Layoutable = layoutable
		ws.OriginX, ws.OriginY = layoutable.X, layoutable.Y
		ws.Width, ws.Height = layoutable.W, layoutable.H
	} else {
		ws.OriginX, ws.OriginY = 0, 0
		ws.Width, ws.Height = 0, 0
	}
	return ws
}

func (dr *DragAndResize) selectedOrNew(tag event.Tag) *DragState {
	for _, ws := range dr.Candidates {
		if ws.Tag == tag {
			return ws
		}
	}
	ws := &DragState{Tag: tag}
	dr.Candidates = append(dr.Candidates, ws)
	return ws
}

// SelectDragState sets the selected drag state and activates it.
func (dr *DragAndResize) SelectDragState(tag event.Tag) *DragState {
	ws := dr.selectedOrNew(tag)
	ws.Active = true
	dr.Selected = ws
	return ws
}

// ClearDragStateSelection clears the current drag-state selection.
func (dr *DragAndResize) ClearDragStateSelection() {
	dr.Selected = nil
}

// ToRegistry converts current candidates into a DragRegistry view.
func (dr *DragAndResize) ToRegistry() *DragRegistry {
	reg := NewDragRegistry(dr.Tag)
	for _, ws := range dr.Candidates {
		reg.AddObject(ws.Tag, &DragObject{
			ID:     ws.Tag,
			OrigX:  ws.OriginX,
			OrigY:  ws.OriginY,
			Width:  ws.Width,
			Height: ws.Height,
		})
	}
	return reg
}

// SyncDragStateFromObject copies object geometry into an existing DragState candidate.
func (dr *DragAndResize) SyncDragStateFromObject(ws *DragState, obj *DragObject) {
	if ws == nil || obj == nil {
		return
	}
	ws.OriginX = obj.OrigX
	ws.OriginY = obj.OrigY
	ws.Width = obj.Width
	ws.Height = obj.Height
	if ws.Layoutable != nil {
		ws.Layoutable.X = obj.OrigX
		ws.Layoutable.Y = obj.OrigY
		ws.Layoutable.W = obj.Width
		ws.Layoutable.H = obj.Height
	}
}

// LayoutCandidate wires a pointer rect for a candidate so pointer events
// route to it. Call it for each candidate's visual frame.
// It also exposes resize handles if the candidate has enough size.
func (dr *DragAndResize) LayoutCandidate(gtx layout.Context, ws *DragState) {
	ws.Active = true
	event.Op(gtx.Ops, ws)
	clip.Rect(image.Rect(
		int(ws.OriginX), int(ws.OriginY),
		int(ws.OriginX+ws.Width), int(ws.OriginY+ws.Height),
	)).Push(gtx.Ops).Pop()

	// Register all 8 resize handles.
	hs := dr.HandleSize
	x, y, w, h := int(ws.OriginX), int(ws.OriginY), int(ws.Width), int(ws.Height)
	dr.layoutEdgeHandle(gtx, ws, ResizeLeft, x, y, w, h, hs)
	dr.layoutEdgeHandle(gtx, ws, ResizeRight, x, y, w, h, hs)
	dr.layoutEdgeHandle(gtx, ws, ResizeTop, x, y, w, h, hs)
	dr.layoutEdgeHandle(gtx, ws, ResizeBottom, x, y, w, h, hs)
	dr.layoutCornerHandle(gtx, ws, ResizeTopLeft, x, y, w, h, hs)
	dr.layoutCornerHandle(gtx, ws, ResizeTopRight, x, y, w, h, hs)
	dr.layoutCornerHandle(gtx, ws, ResizeBottomLeft, x, y, w, h, hs)
	dr.layoutCornerHandle(gtx, ws, ResizeBottomRight, x, y, w, h, hs)
}

// layoutEdgeHandle adds pointer handling for one edge resize handle.
func (dr *DragAndResize) layoutEdgeHandle(gtx layout.Context, ws *DragState, mode ResizeState, x, y, w, h, size int) {
	var r image.Rectangle
	switch mode {
	case ResizeLeft:
		r = image.Rect(x, y, x+size, y+h)
	case ResizeRight:
		r = image.Rect(x+w-size, y, x+w, y+h)
	case ResizeTop:
		r = image.Rect(x, y, x+w, y+size)
	case ResizeBottom:
		r = image.Rect(x, y+h-size, x+w, y+h)
	default:
		return
	}
	event.Op(gtx.Ops, ws)
	clip.Rect(r).Push(gtx.Ops).Pop()
}

// layoutCornerHandle adds pointer handling for one corner resize handle.
func (dr *DragAndResize) layoutCornerHandle(gtx layout.Context, ws *DragState, mode ResizeState, x, y, w, h, size int) {
	s := size + 4 // corners are slightly larger
	var r image.Rectangle
	switch mode {
	case ResizeTopLeft:
		r = image.Rect(x, y, x+s, y+s)
	case ResizeTopRight:
		r = image.Rect(x+w-s, y, x+w, y+s)
	case ResizeBottomLeft:
		r = image.Rect(x, y+h-s, x+s, y+h)
	case ResizeBottomRight:
		r = image.Rect(x+w-s, y+h-s, x+w, y+h)
	default:
		return
	}
	event.Op(gtx.Ops, ws)
	clip.Rect(r).Push(gtx.Ops).Pop()
}

// HitTestResizeHandle checks if point p is within a resize handle of the candidate.
// Returns the ResizeState if hit, or ResizeNone.
func (dr *DragAndResize) HitTestResizeHandle(ws *DragState, p f32.Point) ResizeState {
	hs := float32(dr.HandleSize)
	chs := hs + 4 // corner handles are larger
	x, y, w, h := ws.OriginX, ws.OriginY, ws.Width, ws.Height

	// Check corners first (they have priority over edges).
	// Top-left
	if p.X >= x && p.X <= x+chs && p.Y >= y && p.Y <= y+chs {
		return ResizeTopLeft
	}
	// Top-right
	if p.X >= x+w-chs && p.X <= x+w && p.Y >= y && p.Y <= y+chs {
		return ResizeTopRight
	}
	// Bottom-left
	if p.X >= x && p.X <= x+chs && p.Y >= y+h-chs && p.Y <= y+h {
		return ResizeBottomLeft
	}
	// Bottom-right
	if p.X >= x+w-chs && p.X <= x+w && p.Y >= y+h-chs && p.Y <= y+h {
		return ResizeBottomRight
	}
	// Left edge
	if p.X >= x && p.X <= x+hs && p.Y > y+chs && p.Y < y+h-chs {
		return ResizeLeft
	}
	// Right edge
	if p.X >= x+w-hs && p.X <= x+w && p.Y > y+chs && p.Y < y+h-chs {
		return ResizeRight
	}
	// Top edge
	if p.X > x+chs && p.X < x+w-chs && p.Y >= y && p.Y <= y+hs {
		return ResizeTop
	}
	// Bottom edge
	if p.X > x+chs && p.X < x+w-chs && p.Y >= y-hs && p.Y <= y+h {
		return ResizeBottom
	}
	return ResizeNone
}

// UpdateProcessPointer drains pointer events for all candidates and updates
// DragDelta + geometry for the selected one. For resize handles it
// adjusts Width/Height and/or OriginX/OriginY instead of just moving.
func (dr *DragAndResize) UpdateProcessPointer(gtx layout.Context) {
	for _, ws := range dr.Candidates {
		for {
			ev, ok := gtx.Event(dr.PtrFilter)
			if !ok {
				break
			}
			if ev, ok := ev.(pointer.Event); ok && ws.Active {
				switch ev.Kind {
				case pointer.Press:
					ws.PressedAt = ev.Position
					ws.PressedAtOriginX = ws.OriginX
					ws.PressedAtOriginY = ws.OriginY
					ws.LastPointer = ev.Position
					ws.DragStarted = false
					ws.ResizeState = dr.HitTestResizeHandle(ws, ev.Position)
				case pointer.Drag:
					ws.LastPointer = ev.Position
					delta := ev.Position.Sub(ws.PressedAt)
					// Apply drag threshold for movement (not resize).
					if ws.ResizeState == ResizeNone && !ws.DragStarted {
						if absF32(delta.X) < dr.DragThreshold && absF32(delta.Y) < dr.DragThreshold {
							continue
						}
						ws.DragStarted = true
					}
					dr.applyDragDelta(ws, delta)
				case pointer.Release, pointer.Cancel:
					ws.LastPointer = ev.Position
					if ev.Kind == pointer.Cancel {
						dr.ClearDragStateSelection()
						return
					}
					ws.DragStarted = false
					ws.ResizeState = ResizeNone
				}
			}
		}
	}
}

func absF32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

func (dr *DragAndResize) applyDragDelta(ws *DragState, delta f32.Point) {
	minW := dr.MinSize
	if minW < 1 {
		minW = 10
	}
	minH := minW

	switch ws.ResizeState {
	case ResizeLeft:
		newX := ws.PressedAtOriginX + delta.X
		newW := ws.Width + (ws.OriginX - newX)
		if newW >= minW {
			ws.OriginX = newX
			ws.Width = newW
		}
	case ResizeRight:
		newW := ws.Width + delta.X
		if newW >= minW {
			ws.Width = newW
		}
	case ResizeTop:
		newY := ws.PressedAtOriginY + delta.Y
		newH := ws.Height + (ws.OriginY - newY)
		if newH >= minH {
			ws.OriginY = newY
			ws.Height = newH
		}
	case ResizeBottom:
		newH := ws.Height + delta.Y
		if newH >= minH {
			ws.Height = newH
		}
	case ResizeTopLeft:
		newX := ws.PressedAtOriginX + delta.X
		newY := ws.PressedAtOriginY + delta.Y
		newW := ws.Width + (ws.OriginX - newX)
		newH := ws.Height + (ws.OriginY - newY)
		if newW >= minW {
			ws.OriginX = newX
			ws.Width = newW
		}
		if newH >= minH {
			ws.OriginY = newY
			ws.Height = newH
		}
	case ResizeTopRight:
		newY := ws.PressedAtOriginY + delta.Y
		newW := ws.Width + delta.X
		newH := ws.Height + (ws.OriginY - newY)
		if newW >= minW {
			ws.Width = newW
		}
		if newH >= minH {
			ws.OriginY = newY
			ws.Height = newH
		}
	case ResizeBottomLeft:
		newX := ws.PressedAtOriginX + delta.X
		newW := ws.Width + (ws.OriginX - newX)
		newH := ws.Height + delta.Y
		if newW >= minW {
			ws.OriginX = newX
			ws.Width = newW
		}
		if newH >= minH {
			ws.Height = newH
		}
	case ResizeBottomRight:
		newW := ws.Width + delta.X
		newH := ws.Height + delta.Y
		if newW >= minW {
			ws.Width = newW
		}
		if newH >= minH {
			ws.Height = newH
		}
	default:
		// No resize handle — move the candidate.
		ws.OriginX += delta.X
		ws.OriginY += delta.Y
		// Clamp to canvas bounds.
		if dr.CanvasWidth > 0 {
			if ws.OriginX < 0 {
				ws.OriginX = 0
			}
			if ws.OriginX+ws.Width > dr.CanvasWidth {
				ws.OriginX = dr.CanvasWidth - ws.Width
			}
		}
		if dr.CanvasHeight > 0 {
			if ws.OriginY < 0 {
				ws.OriginY = 0
			}
			if ws.OriginY+ws.Height > dr.CanvasHeight {
				ws.OriginY = dr.CanvasHeight - ws.Height
			}
		}
	}
	ws.DragDelta = f32.Point{}
}

// SelectedRect returns the selected candidate geometry and whether it is linked to a Layoutable.
func (dr *DragAndResize) SelectedRect() (x, y, w, h float32, hasLayoutable bool) {
	if dr.Selected == nil {
		return 0, 0, 0, 0, false
	}
	ws := dr.Selected
	if ws.Layoutable != nil {
		x, y, w, h = ws.Layoutable.Rect()
		return x, y, w, h, true
	}
	return ws.OriginX, ws.OriginY, ws.Width, ws.Height, false
}

// CandidatesRectangle helps hit-testing all registered candidates by their rectangles.
// Returns rectangles in index order (not depth order).
func (dr *DragAndResize) CandidatesRectangle() []Rectangle {
	rects := make([]Rectangle, 0, len(dr.Candidates))
	for _, ws := range dr.Candidates {
		rects = append(rects, Rectangle{
			Min: f32.Point{X: ws.OriginX, Y: ws.OriginY},
			Max: f32.Point{X: ws.OriginX + ws.Width, Y: ws.OriginY + ws.Height},
		})
	}
	return rects
}

// HitTestCandidate returns the first candidate containing point p, searching
// in reverse candidate order (last added = topmost by default).
// Returns nil if no candidate contains p.
func (dr *DragAndResize) HitTestCandidate(p f32.Point) *DragState {
	for i := len(dr.Candidates) - 1; i >= 0; i-- {
		ws := dr.Candidates[i]
		if p.X >= ws.OriginX && p.X <= ws.OriginX+ws.Width &&
			p.Y >= ws.OriginY && p.Y <= ws.OriginY+ws.Height {
			return ws
		}
	}
	return nil
}

// ─── Rectangle ────────────────────────────────────────────────────────────────

// Rectangle is a simple 2D rectangle for hit testing.
type Rectangle struct {
	Min, Max f32.Point
}

// Contains reports whether p is inside the rectangle.
func (r Rectangle) Contains(p f32.Point) bool {
	return p.X >= r.Min.X && p.X <= r.Max.X && p.Y >= r.Min.Y && p.Y <= r.Max.Y
}
