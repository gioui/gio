// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/layout"
	"gioui.org/ui"
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

func (queue) Events(k ui.Key) []ui.Event {
	return nil
}

func (config) Now() time.Time {
	return time.Now()
}

func (config) Px(v ui.Value) int {
	return int(v.V + .5)
}
