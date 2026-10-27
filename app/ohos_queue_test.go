// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos_test

package app

import (
	"testing"
)

func TestOHOSQueueCoalescesMoves(t *testing.T) {
	var q ohosQueue
	for x := 1; x <= 3; x++ {
		if !q.push(ohosEvent{kind: ohosEvTouch, id: 7, action: 1, x: float32(x)}) {
			t.Fatal("move must be accepted")
		}
	}
	if len(q.items) != 1 || q.items[0].x != 3 {
		t.Fatalf("moves must coalesce to the latest: %#v", q.items)
	}
	q.push(ohosEvent{kind: ohosEvTouch, id: 7, action: 0, x: 9})
	q.push(ohosEvent{kind: ohosEvTouch, id: 7, action: 1, x: 10})
	if len(q.items) != 3 {
		t.Fatalf("press/move must not coalesce with the earlier move: %#v", q.items)
	}
}

func TestOHOSQueueKeepsCriticalWhenFull(t *testing.T) {
	var q ohosQueue
	for i := 0; i < ohosQueueLimit; i++ {
		q.push(ohosEvent{kind: ohosEvAxis, dx: float32(i)})
	}
	if len(q.items) != ohosQueueLimit {
		t.Fatalf("queue len = %d", len(q.items))
	}
	if !q.push(ohosEvent{kind: ohosEvLifecycle, life: ohosDestroyed}) {
		t.Fatal("critical event must be accepted")
	}
	found := false
	for _, e := range q.items {
		if e.kind == ohosEvLifecycle {
			found = true
		}
	}
	if !found {
		t.Fatal("critical event dropped")
	}
	if len(q.items) != ohosQueueLimit {
		t.Fatalf("critical event must replace a droppable event, len=%d", len(q.items))
	}
}

func TestOHOSQueueCoalescesFrameAndInsets(t *testing.T) {
	var q ohosQueue
	q.push(ohosEvent{kind: ohosEvFrame})
	q.push(ohosEvent{kind: ohosEvFrame})
	if len(q.items) != 1 {
		t.Fatalf("frames must coalesce: %#v", q.items)
	}
	q.push(ohosEvent{kind: ohosEvInsets, top: 1})
	q.push(ohosEvent{kind: ohosEvInsets, top: 2})
	if len(q.items) != 2 || q.items[1].top != 2 {
		t.Fatalf("insets must replace: %#v", q.items)
	}
}

func TestOHOSQueueTakeRecycle(t *testing.T) {
	var q ohosQueue
	q.push(ohosEvent{kind: ohosEvKey, code: 1})
	q.push(ohosEvent{kind: ohosEvKey, code: 2})
	batch := q.take()
	if len(batch) != 2 || batch[0].code != 1 || batch[1].code != 2 {
		t.Fatalf("take order: %#v", batch)
	}
	q.push(ohosEvent{kind: ohosEvKey, code: 3})
	if len(q.items) != 1 || q.items[0].code != 3 {
		t.Fatalf("new items must not alias the taken batch: %#v", q.items)
	}
	q.recycle(batch)
	if len(q.spare) != 2 || q.spare[0].kind != ohosEvNone {
		t.Fatalf("batch must be cleared and recycled: %#v", q.spare)
	}
}

func TestOHOSQueueDropsOldestInput(t *testing.T) {
	var q ohosQueue
	for i := 0; i < ohosQueueLimit; i++ {
		q.push(ohosEvent{kind: ohosEvAxis, dx: float32(i)})
	}
	q.push(ohosEvent{kind: ohosEvAxis, dx: 999})
	if len(q.items) != ohosQueueLimit {
		t.Fatalf("len = %d", len(q.items))
	}
	if q.dropped != 1 {
		t.Fatalf("dropped = %d", q.dropped)
	}
	if q.items[len(q.items)-1].dx != 999 {
		t.Fatal("newest axis event missing")
	}
	if q.items[0].dx != 1 {
		t.Fatalf("oldest input must be dropped, first=%v", q.items[0].dx)
	}
}
