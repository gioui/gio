package layout_test

import (
	"fmt"
	"image"
	"time"

	"gioui.org/ui"
	"gioui.org/ui/input"
	"gioui.org/ui/layout"
)

type queue struct{}

type config struct{}

var q queue
var cfg = new(config)

func ExampleInset() {
	ops := new(ui.Ops)

	// Loose constraints with no minimal size.
	var cs layout.Constraints
	cs.Width.Max = 100
	cs.Height.Max = 100

	// Inset all edges by 10.
	inset := layout.UniformInset(ui.Dp(10))
	dims := inset.Layout(cfg, ops, cs, func(cs layout.Constraints) layout.Dimensions {
		// Lay out a 50x50 sized widget.
		dims := layoutWidget(50, 50, cs)
		fmt.Println(dims.Size)
		return dims
	})

	fmt.Println(dims.Size)

	// Output:
	// (50,50)
	// (70,70)
}

func ExampleAlign() {
	ops := new(ui.Ops)

	// Rigid constraints with both minimum and maximum set.
	cs := layout.RigidConstraints(image.Point{X: 100, Y: 100})

	align := layout.Align{Alignment: layout.Center}
	dims := align.Layout(ops, cs, func(cs layout.Constraints) layout.Dimensions {
		// Lay out a 50x50 sized widget.
		dims := layoutWidget(50, 50, cs)
		fmt.Println(dims.Size)
		return dims
	})

	fmt.Println(dims.Size)

	// Output:
	// (50,50)
	// (100,100)
}

func ExampleFlex() {
	ops := new(ui.Ops)

	cs := layout.RigidConstraints(image.Point{X: 100, Y: 100})

	flex := layout.Flex{}
	flex.Init(ops, cs)

	// Rigid 10x10 widget.
	child1 := flex.Rigid(func(cs layout.Constraints) layout.Dimensions {
		fmt.Printf("Rigid: %v\n", cs.Width)
		return layoutWidget(10, 10, cs)
	})

	// Child with 50% space allowance.
	child2 := flex.Flexible(0.5, func(cs layout.Constraints) layout.Dimensions {
		fmt.Printf("50%%: %v\n", cs.Width)
		return layoutWidget(10, 10, cs)
	})

	flex.Layout(child1, child2)

	// Output:
	// Rigid: {0 100}
	// 50%: {0 45}
}

func ExampleStack() {
	ops := new(ui.Ops)

	cs := layout.RigidConstraints(image.Point{X: 100, Y: 100})

	stack := layout.Stack{}
	stack.Init(ops, cs)

	// Rigid 50x50 widget.
	child1 := stack.Rigid(func(cs layout.Constraints) layout.Dimensions {
		return layoutWidget(50, 50, cs)
	})

	// Force widget to the same size as the first.
	child2 := stack.Expand(func(cs layout.Constraints) layout.Dimensions {
		fmt.Printf("Expand: %v\n", cs)
		return layoutWidget(10, 10, cs)
	})

	stack.Layout(child1, child2)

	// Output:
	// Expand: {{50 50} {50 50}}
}

func ExampleList() {
	ops := new(ui.Ops)

	cs := layout.RigidConstraints(image.Point{X: 100, Y: 100})

	// The list is 1e6 elements, but only 5 fit the constraints.
	const listLen = 1e6

	var list layout.List
	count := 0
	list.Layout(cfg, q, ops, cs, listLen, func(cs layout.Constraints, i int) layout.Dimensions {
		count++
		return layoutWidget(20, 20, cs)
	})

	fmt.Println(count)

	// Output:
	// 5
}

func layoutWidget(width, height int, cs layout.Constraints) layout.Dimensions {
	return layout.Dimensions{
		Size: image.Point{
			X: width,
			Y: height,
		},
	}
}

func (config) Now() time.Time {
	return time.Now()
}

func (config) Px(v ui.Value) int {
	return int(v.V + .5)
}

func (queue) Next(k input.Key) (input.Event, bool) {
	return nil, false
}
