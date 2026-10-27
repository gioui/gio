// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos_test

package app

import "testing"

func BenchmarkOHOSQueuePush(b *testing.B) {
	var q ohosQueue
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		q.items = q.items[:0]
		q.push(ohosEvent{kind: ohosEvMouse, action: 2, x: float32(i)})
	}
}

func BenchmarkOHOSQueuePushFull(b *testing.B) {
	var q ohosQueue
	for i := 0; i < ohosQueueLimit; i++ {
		q.push(ohosEvent{kind: ohosEvAxis, dx: float32(i)})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		q.push(ohosEvent{kind: ohosEvAxis, dx: float32(i)})
	}
}

func BenchmarkOHOSQueueTakeRecycle(b *testing.B) {
	var q ohosQueue
	for i := 0; i < ohosQueueLimit; i++ {
		q.push(ohosEvent{kind: ohosEvTouch, id: i, action: 1, x: float32(i)})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		batch := q.take()
		if len(batch) == 0 {
			b.Fatal("empty batch")
		}
		q.recycle(batch)
		q.items = append(q.items, ohosEvent{kind: ohosEvTouch, id: 1, action: 1})
	}
}
