// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony || gio_ohos_test

package app

import (
	"image"
	"image/color"
)

// ohosConfigDelta is the set of window properties that changed and must be
// pushed to the platform. Size is deliberately absent: it is owned by the
// XComponent surface.
type ohosConfigDelta struct {
	ModeSet        bool
	Mode           WindowMode
	TitleSet       bool
	Title          string
	OrientationSet bool
	Orientation    Orientation
	StatusSet      bool
	Status         colorNRGBA
	NavigationSet  bool
	Navigation     colorNRGBA
	TopMostSet     bool
	TopMost        bool
	LimitsSet      bool
	Min, Max       image.Point
}

type colorNRGBA = color.NRGBA

func planOHOSConfig(old, new Config) ohosConfigDelta {
	var d ohosConfigDelta
	if old.Mode != new.Mode {
		d.ModeSet, d.Mode = true, new.Mode
	}
	if old.Title != new.Title {
		d.TitleSet, d.Title = true, new.Title
	}
	if old.Orientation != new.Orientation {
		d.OrientationSet, d.Orientation = true, new.Orientation
	}
	if old.StatusColor != new.StatusColor {
		d.StatusSet, d.Status = true, new.StatusColor
	}
	if old.NavigationColor != new.NavigationColor {
		d.NavigationSet, d.Navigation = true, new.NavigationColor
	}
	if old.TopMost != new.TopMost {
		d.TopMostSet, d.TopMost = true, new.TopMost
	}
	if old.MinSize != new.MinSize || old.MaxSize != new.MaxSize {
		d.LimitsSet, d.Min, d.Max = true, new.MinSize, new.MaxSize
	}
	return d
}

func (d ohosConfigDelta) empty() bool {
	return !d.ModeSet && !d.TitleSet && !d.OrientationSet && !d.StatusSet &&
		!d.NavigationSet && !d.TopMostSet && !d.LimitsSet
}

// ohosHost is the platform side of the OHOS backend. The real
// implementation calls the ABI 3 host table; tests use a recorder.
type ohosHost interface {
	Apply(ohosConfigDelta)
	HostSetAnimating(bool)
	RequestClose() bool
	RaiseWindow()
	CenterWindow()
	StartWindowMove()
}

func ohosColor(c colorNRGBA) uint32 {
	return uint32(c.A)<<24 | uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B)
}

func ohosWindowMode(mode WindowMode) int {
	switch mode {
	case Fullscreen:
		return 1
	case Minimized:
		return 2
	case Maximized:
		return 3
	default:
		return 0
	}
}

func ohosOrientation(o Orientation) int {
	switch o {
	case LandscapeOrientation:
		return 1
	case PortraitOrientation:
		return 2
	default:
		return 0
	}
}
