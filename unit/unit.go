// SPDX-License-Identifier: Unlicense OR MIT

/*

Package unit implements device independent units and values.

A Value is a value with a Unit attached.

Device independent pixel, or dp, is the unit for sizes independent of
the underlying display device.

Scaled pixels, or sp, is the unit for text sizes. An sp is like dp with
text scaling applied.

Finally, pixels, or px, is the unit for display dependent pixels. Their
size vary between platforms and displays.

To maintain a constant visual size across platforms and displays, always
use dps or sps to define user interfaces. Only use pixels for derived
values.

*/
package unit

import "fmt"

// Value is a value with a unit.
type Value struct {
	V float32
	U Unit
}

// Unit represents a unit for a Value.
type Unit uint8

// Converter converts Values to pixels.
type Converter interface {
	Px(v Value) int
}

const (
	// UnitPx represent device pixels in the resolution of
	// the underlying display.
	UnitPx Unit = iota
	// UnitDp represents device independent pixels. 1 dp will
	// have the same apparent size across platforms and
	// display resolutions.
	UnitDp
	// UnitSp is like UnitDp but for font sizes.
	UnitSp
)

// Px returns the Value for v device pixels.
func Px(v float32) Value {
	return Value{V: v, U: UnitPx}
}

// Px returns the Value for v device independent
// pixels.
func Dp(v float32) Value {
	return Value{V: v, U: UnitDp}
}

// Sp returns the Value for v scaled dps.
func Sp(v float32) Value {
	return Value{V: v, U: UnitSp}
}

// Scale returns the value scaled by s.
func (v Value) Scale(s float32) Value {
	v.V *= s
	return v
}

func (v Value) String() string {
	return fmt.Sprintf("%g%s", v.V, v.U)
}

func (u Unit) String() string {
	switch u {
	case UnitPx:
		return "px"
	case UnitDp:
		return "dp"
	case UnitSp:
		return "sp"
	default:
		panic("unknown unit")
	}
}

// Add a list of Values.
func Add(c Converter, values ...Value) Value {
	var sum Value
	for _, v := range values {
		sum, v = compatible(c, sum, v)
		sum.V += v.V
	}
	return sum
}

// Max returns the maximum of a list of Values.
func Max(c Converter, values ...Value) Value {
	var max Value
	for _, v := range values {
		max, v = compatible(c, max, v)
		if v.V > max.V {
			max.V = v.V
		}
	}
	return max
}

func compatible(c Converter, v1, v2 Value) (Value, Value) {
	if v1.U == v2.U {
		return v1, v2
	}
	if v1.V == 0 {
		v1.U = v2.U
		return v1, v2
	}
	if v2.V == 0 {
		v2.U = v1.U
		return v1, v2
	}
	return Px(float32(c.Px(v1))), Px(float32(c.Px(v2)))
}
