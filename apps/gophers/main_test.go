// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"image"
	"testing"
	"time"

	"gioui.org/ui"
	"gioui.org/ui/input"
	"gioui.org/ui/layout"
)

type queue struct{}

type config struct{}

func BenchmarkUI(b *testing.B) {
	fetch := func(_ string) {}
	u := newUI(fetch)
	ops := new(ui.Ops)
	q := new(queue)
	c := new(config)
	ctx := new(layout.Context)
	ctx.Constraints = layout.RigidConstraints(image.Point{800, 600})
	for i := 0; i < b.N; i++ {
		ops.Reset()
		u.Layout(c, q, ops, ctx)
	}
}

func (queue) Next(k input.Key) (input.Event, bool) {
	return nil, false
}

func (config) Now() time.Time {
	return time.Now()
}

func (config) Px(v ui.Value) int {
	return int(v.V + .5)
}
