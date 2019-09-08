// SPDX-License-Identifier: Unlicense OR MIT

package ui

import "fmt"

// Value is a value with a unit.
type Value struct {
	V float32
	U Unit
}

// Unit represents a unit for a Value.
type Unit uint8

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
func Add(c Config, values ...Value) Value {
	var sum Value
	for _, v := range values {
		sum, v = compatible(c, sum, v)
		sum.V += v.V
	}
	return sum
}

// Max returns the maximum of a list of Values.
func Max(c Config, values ...Value) Value {
	var max Value
	for _, v := range values {
		max, v = compatible(c, max, v)
		if v.V > max.V {
			max.V = v.V
		}
	}
	return max
}

func compatible(c Config, v1, v2 Value) (Value, Value) {
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
