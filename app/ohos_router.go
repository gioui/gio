// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony || gio_ohos_test

package app

// ohosWindowRouter maps platform window IDs to windows. It is not
// synchronized: callers hold ohosState.mu. IDs are allocated by Go and are
// passed to the shim through the v4 create/select callbacks.
type ohosWindowRouter[T any] struct {
	windows   map[uint64]T
	order     []uint64
	staging   map[uint64][]ohosEvent
	retired   map[uint64]bool
	defaultID uint64
	next      uint64
}

func (r *ohosWindowRouter[T]) init() {
	if r.windows == nil {
		r.windows = make(map[uint64]T)
		r.staging = make(map[uint64][]ohosEvent)
		r.retired = make(map[uint64]bool)
	}
}

// assign returns a fresh non-zero window ID.
func (r *ohosWindowRouter[T]) assign() uint64 {
	r.init()
	r.next++
	if r.next == 0 {
		r.next++
	}
	return r.next
}

// register adds target and returns events staged for id before it existed.
func (r *ohosWindowRouter[T]) register(id uint64, target T) []ohosEvent {
	r.init()
	if _, ok := r.windows[id]; !ok {
		r.order = append(r.order, id)
	}
	r.windows[id] = target
	delete(r.retired, id)
	if r.defaultID == 0 {
		r.defaultID = id
	}
	// Events delivered through the legacy (ID-less) API before the first
	// window belong to that window.
	pending := append([]ohosEvent(nil), r.staging[0]...)
	pending = append(pending, r.staging[id]...)
	delete(r.staging, 0)
	delete(r.staging, id)
	return pending
}

func (r *ohosWindowRouter[T]) unregister(id uint64) {
	delete(r.windows, id)
	if r.retired == nil {
		r.retired = make(map[uint64]bool)
	}
	r.retired[id] = true
	delete(r.staging, id)
	for i, v := range r.order {
		if v == id {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
	if r.defaultID == id {
		r.defaultID = 0
		if len(r.order) > 0 {
			r.defaultID = r.order[0]
		}
	}
}

// get resolves 0 to the default window.
func (r *ohosWindowRouter[T]) get(id uint64) (T, bool) {
	if id == 0 {
		id = r.defaultID
	}
	v, ok := r.windows[id]
	return v, ok
}

func (r *ohosWindowRouter[T]) defaultTarget() (T, bool) {
	return r.get(0)
}

func (r *ohosWindowRouter[T]) all() []T {
	out := make([]T, 0, len(r.order))
	for _, id := range r.order {
		if v, ok := r.windows[id]; ok {
			out = append(out, v)
		}
	}
	return out
}

// stage buffers events for a window that has not been created yet.
func (r *ohosWindowRouter[T]) stage(id uint64, e ohosEvent) bool {
	r.init()
	if r.retired[id] {
		return false
	}
	if len(r.staging[id]) >= ohosQueueLimit && !ohosEventCritical(e.kind) {
		return false
	}
	r.staging[id] = append(r.staging[id], e)
	return true
}
