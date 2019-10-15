// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"fmt"
	"strconv"

	"gioui.org/unit"
)

type formatState struct {
	current int
	orig    string
	expr    string
	skip    int
}

type formatError string

// Format lays out widgets according to a format string, similar to
// how fmt.Printf interpolates a string.
//
// The format string is an epxression where layouts are similar to
// function calls, and the underscore denotes a widget from the
// arguments. The ith _ invokes the ith widget from the arguments.
//
// If the layout format is invalid, Format panics with an error where
// a cross, ✗, marks the error position.
//
// For example,
//
//   layout.Format(gtx, "inset(8dp, _)", w)
//
// is equivalent to
//
//   layout.UniformInset(unit.Dp(8)).Layout(gtx, w)
//
// Available layouts:
//
//   inset(insets, widget) applies Inset to widget. Insets are either:
//   one value for uniform insets; two values for top/bottom and
//   right/left insets; three values for top, bottom and right/left
//   insets; or four values for top, right, bottom, left insets.
//
//   <direction>(widget) applies a directed ALign to widget. Direction
//   is one of north, northeast, east, southeast, south, southwest, west,
//   northwest, center.
//
//   hexpand/vexpand(widget) forces the horizontalor or vertical
//   constraints to their maximum before laying out widget.
//
//   hcap/vcap(<size>, widget) caps the maximum horizontal or vertical
//   constraints to size.
//
//   hflex/vflex(<alignment>, children...) lays out children with a
//   horizontal or vertical Flex. Each rigid child must be on the form
//   r(widget), and each flex child on the form f(<weight>, widget).
//   If alignment is specified, it must be one of: start, middle, end,
//   baseline. The default alignment is start.
//
//   stack(<alignment>, children) lays out children with a Stack. Each
//   Rigid child must be on the form r(widget), and each expand child
//   on the form e(widget).
//   If alignment is specified it must be one of the directions listed
//   above.
func Format(gtx *Context, format string, widgets ...Widget) {
	if format == "" {
		return
	}
	state := formatState{
		orig: format,
		expr: format,
	}
	defer func() {
		if err := recover(); err != nil {
			if _, ok := err.(formatError); !ok {
				panic(err)
			}
			pos := len(state.orig) - len(state.expr)
			msg := state.orig[:pos] + "✗" + state.orig[pos:]
			panic(fmt.Errorf("Format: %s:%d: %s", msg, pos, err))
		}
	}()
	formatExpr(gtx, &state, widgets)
}

func formatExpr(gtx *Context, state *formatState, widgets []Widget) {
	switch peek(state) {
	case '_':
		formatWidget(gtx, state, widgets)
	default:
		formatLayout(gtx, state, widgets)
	}
}

func formatLayout(gtx *Context, state *formatState, widgets []Widget) {
	name := parseName(state)
	if name == "" {
		errorf("missing layout name")
	}
	expect(state, "(")
	f := func() {
		formatExpr(gtx, state, widgets)
	}
	align, ok := dirFor(name)
	if ok {
		Align(align).Layout(gtx, f)
		expect(state, ")")
		return
	}
	switch name {
	case "inset":
		in := parseInset(gtx, state, widgets)
		in.Layout(gtx, f)
	case "hflex":
		formatFlex(gtx, Horizontal, state, widgets)
	case "vflex":
		formatFlex(gtx, Vertical, state, widgets)
	case "stack":
		formatStack(gtx, state, widgets)
	case "hexp":
		cs := gtx.Constraints
		cs.Width.Min = cs.Width.Max
		ctxLayout(gtx, cs, func() {
			formatExpr(gtx, state, widgets)
		})
	case "vexp":
		cs := gtx.Constraints
		cs.Height.Min = cs.Height.Max
		ctxLayout(gtx, cs, func() {
			formatExpr(gtx, state, widgets)
		})
	case "hcap":
		w := parseValue(state)
		expect(state, ",")
		cs := gtx.Constraints
		cs.Width.Max = cs.Width.Constrain(gtx.Px(w))
		ctxLayout(gtx, cs, func() {
			formatExpr(gtx, state, widgets)
		})
	case "vcap":
		h := parseValue(state)
		expect(state, ",")
		cs := gtx.Constraints
		cs.Height.Max = cs.Height.Constrain(gtx.Px(h))
		ctxLayout(gtx, cs, func() {
			formatExpr(gtx, state, widgets)
		})
	default:
		errorf("invalid layout %q", name)
	}
	expect(state, ")")
}

func formatWidget(gtx *Context, state *formatState, widgets []Widget) {
	expect(state, "_")
	if i, max := state.current, len(widgets)-1; i > max {
		errorf("widget index %d out of bounds [0;%d]", i, max)
	}
	if state.skip == 0 {
		widgets[state.current]()
	}
	state.current++
}

func formatStack(gtx *Context, state *formatState, widgets []Widget) {
	st := Stack{}
	// Parse alignment, if present.
	switch peek(state) {
	case 'r', 'e', ')':
	default:
		name := parseName(state)
		align, ok := dirFor(name)
		if !ok {
			errorf("invalid stack alignment: %q", name)
		}
		st.Alignment = align
		expect(state, ",")
	}
	var children []StackChild
	// First, lay out rigid children.
	backup := *state
loop:
	for {
		switch peek(state) {
		case ')':
			break loop
		case ',':
			expect(state, ",")
		case 'r':
			expect(state, "r(")
			children = append(children, st.Rigid(gtx, func() {
				formatExpr(gtx, state, widgets)
			}))
			expect(state, ")")
		case 'e':
			expect(state, "e(")
			state.skip++
			formatExpr(gtx, state, widgets)
			children = append(children, StackChild{})
			state.skip--
			expect(state, ")")
		default:
			errorf("invalid flex child")
		}
	}
	// Then, lay out expanded children.
	*state = backup
	child := 0
	for {
		switch peek(state) {
		case ')':
			st.Layout(gtx, children...)
			return
		case ',':
			expect(state, ",")
		case 'r':
			expect(state, "r(")
			state.skip++
			formatExpr(gtx, state, widgets)
			state.skip--
			expect(state, ")")
			child++
		case 'e':
			expect(state, "e(")
			children[child] = st.Expand(gtx, func() {
				formatExpr(gtx, state, widgets)
			})
			expect(state, ")")
			child++
		default:
			errorf("invalid flex child")
		}
	}
}

func formatFlex(gtx *Context, axis Axis, state *formatState, widgets []Widget) {
	fl := Flex{Axis: axis}
	// Parse alignment, if present.
	switch peek(state) {
	case 'r', 'f', ')':
	default:
		name := parseName(state)
		switch name {
		case "start":
			fl.Alignment = Start
		case "middle":
			fl.Alignment = Middle
		case "end":
			fl.Alignment = End
		case "baseline":
			fl.Alignment = Baseline
		default:
			errorf("invalid flex alignment: %q", name)
		}
		expect(state, ",")
	}
	var children []FlexChild
	// First, lay out rigid children.
	backup := *state
loop:
	for {
		switch peek(state) {
		case ')':
			break loop
		case ',':
			expect(state, ",")
		case 'r':
			expect(state, "r(")
			children = append(children, fl.Rigid(gtx, func() {
				formatExpr(gtx, state, widgets)
			}))
			expect(state, ")")
		case 'f':
			expect(state, "f(")
			parseFloat(state)
			expect(state, ",")
			state.skip++
			formatExpr(gtx, state, widgets)
			children = append(children, FlexChild{})
			state.skip--
			expect(state, ")")
		default:
			errorf("invalid flex child")
		}
	}
	// Then, lay out flexible children.
	*state = backup
	child := 0
	for {
		switch peek(state) {
		case ')':
			fl.Layout(gtx, children...)
			return
		case ',':
			expect(state, ",")
		case 'r':
			expect(state, "r(")
			state.skip++
			formatExpr(gtx, state, widgets)
			state.skip--
			expect(state, ")")
			child++
		case 'f':
			expect(state, "f(")
			weight := parseFloat(state)
			expect(state, ",")
			children[child] = fl.Flex(gtx, weight, func() {
				formatExpr(gtx, state, widgets)
			})
			expect(state, ")")
			child++
		default:
			errorf("invalid flex child")
		}
	}
}

func parseInset(gtx *Context, state *formatState, widgets []Widget) Inset {
	v1 := parseValue(state)
	if peek(state) == ',' {
		expect(state, ",")
		return UniformInset(v1)
	}
	v2 := parseValue(state)
	if peek(state) == ',' {
		expect(state, ",")
		return Inset{
			Top:    v1,
			Right:  v2,
			Bottom: v1,
			Left:   v2,
		}
	}
	v3 := parseValue(state)
	if peek(state) == ',' {
		expect(state, ",")
		return Inset{
			Top:    v1,
			Right:  v2,
			Bottom: v3,
			Left:   v2,
		}
	}
	v4 := parseValue(state)
	expect(state, ",")
	return Inset{
		Top:    v1,
		Right:  v2,
		Bottom: v3,
		Left:   v4,
	}
}

func parseValue(state *formatState) unit.Value {
	i := parseFloat(state)
	if len(state.expr) < 2 {
		errorf("missing unit")
	}
	u := state.expr[:2]
	var v unit.Value
	switch u {
	case "dp":
		v = unit.Dp(i)
	case "sp":
		v = unit.Sp(i)
	case "px":
		v = unit.Px(i)
	default:
		errorf("unknown unit")
	}
	state.expr = state.expr[len(u):]
	return v
}

func parseName(state *formatState) string {
	i := 0
	for ; i < len(state.expr); i++ {
		c := state.expr[i]
		switch {
		case c == '(' || c == ',' || c == ')':
			fname := state.expr[:i]
			state.expr = state.expr[i:]
			return fname
		case c < 'a' || 'z' < c:
			errorf("invalid character '%c' in layout name", c)
		}
	}
	state.expr = state.expr[i:]
	errorf("missing ( after layout function")
	return ""
}

func parseFloat(state *formatState) float32 {
	i := 0
	for ; i < len(state.expr); i++ {
		c := state.expr[i]
		if (c < '0' || c > '9') && c != '.' {
			break
		}
	}
	expr := state.expr[:i]
	v, err := strconv.ParseFloat(expr, 32)
	if err != nil {
		errorf("invalid number %q", expr)
	}
	state.expr = state.expr[i:]
	return float32(v)
}

func parseInt(state *formatState) int {
	i := 0
	for ; i < len(state.expr); i++ {
		c := state.expr[i]
		if c < '0' || c > '9' {
			break
		}
	}
	expr := state.expr[:i]
	v, err := strconv.Atoi(expr)
	if err != nil {
		errorf("invalid number %q", expr)
	}
	state.expr = state.expr[i:]
	return v
}

func peek(state *formatState) rune {
	skipWhitespace(state)
	if len(state.expr) == 0 {
		errorf("unexpected end")
	}
	return rune(state.expr[0])
}

func expect(state *formatState, str string) {
	skipWhitespace(state)
	n := len(str)
	if len(state.expr) < n || state.expr[:n] != str {
		errorf("expected %q", str)
	}
	state.expr = state.expr[n:]
}

func skipWhitespace(state *formatState) {
	for len(state.expr) > 0 {
		switch state.expr[0] {
		case '\t', '\n', '\v', '\f', '\r', ' ':
			state.expr = state.expr[1:]
		default:
			return
		}
	}
}

func dirFor(name string) (Direction, bool) {
	var d Direction
	switch name {
	case "center":
		d = Center
	case "northwest":
		d = NW
	case "north":
		d = N
	case "northeeast":
		d = NE
	case "east":
		d = E
	case "southeast":
		d = SE
	case "south":
		d = S
	case "southwest":
		d = SW
	case "west":
		d = W
	default:
		return 0, false
	}
	return d, true
}

func errorf(f string, args ...interface{}) {
	panic(formatError(fmt.Sprintf(f, args...)))
}

func (e formatError) Error() string {
	return string(e)
}
