// SPDX-License-Identifier: Unlicense OR MIT

//go:build darwin && !ios

package app

// magnifyScrollFactor converts NSEvent.magnification into a wheel delta.
//
// gpofd (and the canvas conventions shared by the desktop backends) treat a
// negative Scroll.Y with the Ctrl modifier as "zoom in".  wheelZoom applies
// 1.2^(-Scroll.Y/50) per event, so factor 275 makes a 10% pinch (0.1)
// approximately a 1.1x zoom, matching the previous gpofd behaviour.
const magnifyScrollFactor = 275

// magnifyScroll returns the Scroll.Y value for a magnify delta.  Opening the
// fingers (positive magnification) yields a negative delta, i.e. zoom in.
func magnifyScroll(delta float64) float32 {
	return float32(-delta * magnifyScrollFactor)
}
