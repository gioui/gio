// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos_test

package app

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/io/system"
)

type fakeHost struct {
	deltas    []ohosConfigDelta
	animating []bool
	closes    int
	closeOK   bool
	raises    int
	centers   int
	moves     int
}

func (f *fakeHost) Apply(d ohosConfigDelta) { f.deltas = append(f.deltas, d) }
func (f *fakeHost) HostSetAnimating(b bool) { f.animating = append(f.animating, b) }
func (f *fakeHost) RequestClose() bool      { f.closes++; return f.closeOK }
func (f *fakeHost) RaiseWindow()            { f.raises++ }
func (f *fakeHost) CenterWindow()           { f.centers++ }
func (f *fakeHost) StartWindowMove()        { f.moves++ }

func TestOHOSPlanConfig(t *testing.T) {
	old := Config{}
	newCfg := Config{
		Title:           "hello",
		Mode:            Fullscreen,
		Orientation:     LandscapeOrientation,
		StatusColor:     color.NRGBA{R: 1, A: 255},
		NavigationColor: color.NRGBA{G: 2, A: 255},
		TopMost:         true,
		MinSize:         image.Pt(10, 20),
		MaxSize:         image.Pt(30, 40),
		Size:            image.Pt(999, 999), // must be ignored
	}
	d := planOHOSConfig(old, newCfg)
	if !d.ModeSet || d.Mode != Fullscreen {
		t.Fatalf("mode = %+v", d)
	}
	if !d.TitleSet || d.Title != "hello" {
		t.Fatalf("title = %+v", d)
	}
	if !d.OrientationSet || d.Orientation != LandscapeOrientation {
		t.Fatalf("orientation = %+v", d)
	}
	if !d.StatusSet || !d.NavigationSet || !d.TopMostSet || !d.LimitsSet {
		t.Fatalf("missing flags: %+v", d)
	}
	if d.Min != image.Pt(10, 20) || d.Max != image.Pt(30, 40) {
		t.Fatalf("limits = %+v", d)
	}
	if d.empty() {
		t.Fatal("delta must not be empty")
	}
	if !planOHOSConfig(newCfg, newCfg).empty() {
		t.Fatal("unchanged config must produce an empty delta")
	}
}

func TestOHOSPumpConfigureCallsHost(t *testing.T) {
	p, f, h := newTestPumpWithHost()
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 800, h: 600})
	p.Handle(ohosEvent{kind: ohosEvFrame})
	f.events, f.frames = nil, 0

	p.Configure([]Option{
		Title("hello"),
		StatusColor(color.NRGBA{R: 1, A: 255}),
		MaxSize(100, 200),
	})
	if p.config.Size != image.Pt(800, 600) {
		t.Fatalf("surface size must win, got %v", p.config.Size)
	}
	if len(h.deltas) != 1 {
		t.Fatalf("host deltas = %#v", h.deltas)
	}
	d := h.deltas[0]
	if !d.TitleSet || d.Title != "hello" || !d.StatusSet || !d.LimitsSet {
		t.Fatalf("delta = %+v", d)
	}
	if f.frames != 1 || len(f.events) != 1 {
		t.Fatalf("configure must emit ConfigEvent and request a frame: events=%#v frames=%d", f.events, f.frames)
	}
}

func TestOHOSPumpPerformClose(t *testing.T) {
	p, f, h := newTestPumpWithHost()
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 100, h: 100})
	p.Handle(ohosEvent{kind: ohosEvFrame})
	f.events = nil
	h.closeOK = false
	p.Handle(ohosEvent{kind: ohosEvAction, sysAction: system.ActionClose})
	if len(f.events) != 1 {
		t.Fatalf("unsupported close must fall back to DestroyEvent: %#v", f.events)
	}
	if _, ok := f.events[0].(DestroyEvent); !ok {
		t.Fatalf("want DestroyEvent, got %#v", f.events[0])
	}

	p2, f2, h2 := newTestPumpWithHost()
	p2.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 100, h: 100})
	p2.Handle(ohosEvent{kind: ohosEvFrame})
	f2.events = nil
	h2.closeOK = true
	p2.Handle(ohosEvent{kind: ohosEvAction, sysAction: system.ActionClose})
	if len(f2.events) != 0 {
		t.Fatalf("supported close must wait for lifecycle(destroy): %#v", f2.events)
	}
	if h2.closes != 1 {
		t.Fatalf("close calls = %d", h2.closes)
	}
}

func TestOHOSPumpPerformWindowActions(t *testing.T) {
	p, f, h := newTestPumpWithHost()
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 100, h: 100})
	p.Handle(ohosEvent{kind: ohosEvFrame})
	f.events, f.frames = nil, 0

	p.Handle(ohosEvent{kind: ohosEvAction, sysAction: system.ActionMaximize})
	if p.config.Mode != Maximized {
		t.Fatalf("mode = %v", p.config.Mode)
	}
	if len(h.deltas) != 1 || !h.deltas[0].ModeSet || h.deltas[0].Mode != Maximized {
		t.Fatalf("deltas = %#v", h.deltas)
	}
	if f.frames != 1 {
		t.Fatalf("mode action must request a frame, got %d", f.frames)
	}

	p.Handle(ohosEvent{kind: ohosEvAction, sysAction: system.ActionRaise | system.ActionCenter | system.ActionMove})
	if h.raises != 1 || h.centers != 1 || h.moves != 1 {
		t.Fatalf("actions = %d/%d/%d", h.raises, h.centers, h.moves)
	}
}

func TestOHOSPumpSystemWindowModeAndOrientation(t *testing.T) {
	p, f, h := newTestPumpWithHost()
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 100, h: 100})
	p.Handle(ohosEvent{kind: ohosEvFrame})
	f.events, f.frames = nil, 0
	h.deltas = nil

	p.Handle(ohosEvent{kind: ohosEvWindowMode, a: 1})
	if p.config.Mode != Fullscreen || len(h.deltas) != 1 || !h.deltas[0].ModeSet {
		t.Fatalf("mode event: config=%v deltas=%#v", p.config.Mode, h.deltas)
	}
	p.Handle(ohosEvent{kind: ohosEvOrientation, a: 2})
	if p.config.Orientation != PortraitOrientation || len(h.deltas) != 2 || !h.deltas[1].OrientationSet {
		t.Fatalf("orientation event: config=%v deltas=%#v", p.config.Orientation, h.deltas)
	}
	if f.frames != 1 {
		t.Fatalf("system config changes must coalesce into one frame, got %d", f.frames)
	}
}

func TestOHOSPumpSetAnimatingHost(t *testing.T) {
	p, _, h := newTestPumpWithHost()
	p.Handle(ohosEvent{kind: ohosEvSurfaceCreated, win: 7, w: 100, h: 100})
	p.SetAnimating(true)
	if len(h.animating) != 1 || !h.animating[0] {
		t.Fatalf("host animating = %#v", h.animating)
	}
	p.SetAnimating(false)
	if len(h.animating) != 2 || h.animating[1] {
		t.Fatalf("host animating = %#v", h.animating)
	}
}
