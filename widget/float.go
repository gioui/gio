// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
)

// Float is for selecting a value in a range.
type Float struct {
	Value float32

	drag    gesture.Drag
	pos     float32 // position normalized to [0, 1]
	length  float32
	changed bool
}

// Layout processes events.
func (f *Float) Layout(gtx layout.Context, pointerMargin int, min, max float32) layout.Dimensions {
	size := gtx.Constraints.Min
	f.length = float32(size.X)

	var de *pointer.Event
	for _, e := range f.drag.Events(gtx.Metric, gtx, gesture.Horizontal) {
		if e.Type == pointer.Press || e.Type == pointer.Drag {
			de = &e
		}
	}

	value := f.Value
	if de != nil {
		f.pos = de.Position.X / f.length
		value = min + (max-min)*f.pos
	} else if min != max {
		f.pos = value/(max-min) - min
	}
	// Unconditionally call setValue in case min, max, or value changed.
	f.setValue(value, min, max)

	if f.pos < 0 {
		f.pos = 0
	} else if f.pos > 1 {
		f.pos = 1
	}

	defer op.Push(gtx.Ops).Pop()
	rect := image.Rectangle{Max: size}
	rect.Min.X -= pointerMargin
	rect.Max.X += pointerMargin
	pointer.Rect(rect).Add(gtx.Ops)
	f.drag.Add(gtx.Ops)

	return layout.Dimensions{Size: size}
}

func (f *Float) setValue(value, min, max float32) {
	if min > max {
		min, max = max, min
	}
	if value < min {
		value = min
	} else if value > max {
		value = max
	}
	if f.Value != value {
		f.Value = value
		f.changed = true
	}
}

// Pos reports the selected position.
func (f *Float) Pos() float32 {
	return f.pos * f.length
}

// Changed reports whether the value has changed since
// the last call to Changed.
func (f *Float) Changed() bool {
	changed := f.changed
	f.changed = false
	return changed
}
