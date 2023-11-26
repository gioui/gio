// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"testing"

	"gioui.org/io/pointer"
	"gioui.org/op"
)

func TestNoFilterAllocs(t *testing.T) {
	b := testing.Benchmark(func(b *testing.B) {
		var r Router
		s := r.Source()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			s.Event(pointer.Filter{})
		}
	})
	if allocs := b.AllocsPerOp(); allocs != 0 {
		t.Fatalf("expected 0 AllocsPerOp, got %d", allocs)
	}
}

func TestRouterWakeup(t *testing.T) {
	r := new(Router)
	r.Source().Execute(op.InvalidateCmd{})
	r.Frame(new(op.Ops))
	if _, wake := r.WakeupTime(); !wake {
		t.Errorf("InvalidateCmd did not trigger a redraw")
	}
}
