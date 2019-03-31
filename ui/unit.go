// SPDX-License-Identifier: Unlicense OR MIT

package ui

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

func Px(v float32) Value {
	return Value{V: v, U: UnitPx}
}

func Dp(v float32) Value {
	return Value{V: v, U: UnitDp}
}

func Sp(v float32) Value {
	return Value{V: v, U: UnitSp}
}
