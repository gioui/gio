// SPDX-License-Identifier: Unlicense OR MIT

package ui

type Value struct {
	V float32
	U Unit
}

type Unit uint8

const (
	UnitPx Unit = iota
	UnitDp
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

func (c *Config) Pixels(v Value) float32 {
	switch v.U {
	case UnitPx:
		return v.V
	case UnitDp:
		return c.PxPerDp * v.V
	case UnitSp:
		return c.PxPerSp * v.V
	default:
		panic("unknown unit")
	}
}
