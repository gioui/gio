// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/unit"
)

type queue struct{}

type config struct{}

func BenchmarkUI(b *testing.B) {
	fetch := func(_ string) {}
	u := newUI(fetch)
	cfg := new(config)
	gtx := &layout.Context{
		Queue: new(queue),
	}
	for i := 0; i < b.N; i++ {
		gtx.Reset(cfg, image.Point{800, 600})
		u.Layout(gtx)
	}
}

func (queue) Events(k event.Key) []event.Event {
	return nil
}

func (config) Now() time.Time {
	return time.Now()
}

func (config) Px(v unit.Value) int {
	return int(v.V + .5)
}
