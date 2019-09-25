// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/ui"
	"gioui.org/ui/layout"
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
		gtx.Reset(cfg, layout.RigidConstraints(image.Point{800, 600}))
		u.Layout(gtx)
	}
}

func (queue) Next(k ui.Key) (ui.Event, bool) {
	return nil, false
}

func (config) Now() time.Time {
	return time.Now()
}

func (config) Px(v ui.Value) int {
	return int(v.V + .5)
}
