// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony || gio_ohos_test

package app

// ohosQueueLimit bounds the number of pending platform callbacks. Input
// moves are coalesced and, when the queue is full, the oldest droppable
// event is discarded. Lifecycle, surface and IME events are never dropped.
const ohosQueueLimit = 256

// ohosQueue is a double-buffered event queue owned by one window. Only
// items is touched by producers; take/recycle are used by the loop goroutine.
type ohosQueue struct {
	items   []ohosEvent
	spare   []ohosEvent
	dropped uint64
}

// push appends e, coalescing or dropping low-value events when needed.
// It reports whether the event was accepted.
func (q *ohosQueue) push(e ohosEvent) bool {
	if n := len(q.items); n > 0 {
		last := &q.items[n-1]
		switch e.kind {
		case ohosEvFrame:
			if last.kind == ohosEvFrame {
				return true
			}
		case ohosEvInsets:
			if last.kind == ohosEvInsets {
				*last = e
				return true
			}
		case ohosEvSurfaceChanged:
			if last.kind == ohosEvSurfaceChanged {
				*last = e
				return true
			}
		case ohosEvConfigure, ohosEvInvalidate:
			if last.kind == e.kind {
				*last = e
				return true
			}
		case ohosEvTouch:
			if last.kind == ohosEvTouch && e.action == 1 && last.action == 1 && last.id == e.id {
				*last = e
				return true
			}
		case ohosEvMouse:
			if last.kind == ohosEvMouse && e.action == 2 && last.action == 2 {
				*last = e
				return true
			}
		}
	}
	if len(q.items) < ohosQueueLimit {
		q.items = append(q.items, e)
		return true
	}
	// Full: make room by dropping the oldest low-value event.
	for i, ev := range q.items {
		if ohosEventDroppable(ev.kind) {
			copy(q.items[i:], q.items[i+1:])
			q.items = q.items[:len(q.items)-1]
			q.items = append(q.items, e)
			q.dropped++
			return true
		}
	}
	// Critical events may exceed the soft limit rather than be lost.
	if ohosEventCritical(e.kind) {
		q.items = append(q.items, e)
		return true
	}
	q.dropped++
	return false
}

func ohosEventDroppable(k ohosEventKind) bool {
	switch k {
	case ohosEvTouch, ohosEvMouse, ohosEvAxis, ohosEvFrame:
		return true
	}
	return false
}

func ohosEventCritical(k ohosEventKind) bool {
	switch k {
	case ohosEvSurfaceCreated, ohosEvSurfaceChanged, ohosEvSurfaceDestroyed,
		ohosEvLifecycle, ohosEvKey, ohosEvText, ohosEvFocus,
		ohosEvPreedit, ohosEvIMEDelete, ohosEvIMEMove, ohosEvIMESelection,
		ohosEvIMEEnter, ohosEvIMEKeyboard, ohosEvPickResult, ohosEvPickError,
		ohosEvConfigure, ohosEvInsets, ohosEvInvalidate, ohosEvMetric,
		ohosEvDrop, ohosEvClipboardData, ohosEvSemanticAction, ohosEvPermission:
		return true
	}
	return false
}

// take swaps the active batch out of the queue. The caller must call
// recycle with the returned slice when done.
func (q *ohosQueue) take() []ohosEvent {
	if len(q.items) == 0 {
		return nil
	}
	batch := q.items
	q.items = q.spare[:0]
	q.spare = nil
	return batch
}

// recycle returns a processed batch for reuse.
func (q *ohosQueue) recycle(batch []ohosEvent) {
	for i := range batch {
		batch[i] = ohosEvent{}
	}
	q.spare = batch
}

// clear drops all pending events and cached buffers. It is called on
// destroy and must not race with push (the window is closed under qmu).
func (q *ohosQueue) clear() {
	q.items = nil
	q.spare = nil
}
