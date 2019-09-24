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
	ctx := new(layout.Context)

	// Loose constraints with no minimal size.
	ctx.Constraints.Width.Max = 100
	ctx.Constraints.Height.Max = 100

	// Inset all edges by 10.
	inset := layout.UniformInset(ui.Dp(10))
	inset.Layout(cfg, ops, ctx, func() {
		// Lay out a 50x50 sized widget.
		layoutWidget(ctx, 50, 50)
		fmt.Println(ctx.Dimensions.Size)
	})

	fmt.Println(ctx.Dimensions.Size)

	// Output:
	// (50,50)
	// (70,70)
}

func ExampleAlign() {
	ops := new(ui.Ops)
	ctx := new(layout.Context)

	// Rigid constraints with both minimum and maximum set.
	ctx.Constraints = layout.RigidConstraints(image.Point{X: 100, Y: 100})

	align := layout.Align{Alignment: layout.Center}
	align.Layout(ops, ctx, func() {
		// Lay out a 50x50 sized widget.
		layoutWidget(ctx, 50, 50)
		fmt.Println(ctx.Dimensions.Size)
	})

	fmt.Println(ctx.Dimensions.Size)

	// Output:
	// (50,50)
	// (100,100)
}

func ExampleFlex() {
	ops := new(ui.Ops)
	ctx := new(layout.Context)

	ctx.Constraints = layout.RigidConstraints(image.Point{X: 100, Y: 100})

	flex := layout.Flex{}
	flex.Init(ops, ctx)

	// Rigid 10x10 widget.
	child1 := flex.Rigid(func() {
		fmt.Printf("Rigid: %v\n", ctx.Constraints.Width)
		layoutWidget(ctx, 10, 10)
	})

	// Child with 50% space allowance.
	child2 := flex.Flexible(0.5, func() {
		fmt.Printf("50%%: %v\n", ctx.Constraints.Width)
		layoutWidget(ctx, 10, 10)
	})

	flex.Layout(child1, child2)

	// Output:
	// Rigid: {0 100}
	// 50%: {0 45}
}

func ExampleStack() {
	ops := new(ui.Ops)
	ctx := new(layout.Context)

	ctx.Constraints = layout.RigidConstraints(image.Point{X: 100, Y: 100})

	stack := layout.Stack{}
	stack.Init(ops, ctx)

	// Rigid 50x50 widget.
	child1 := stack.Rigid(func() {
		layoutWidget(ctx, 50, 50)
	})

	// Force widget to the same size as the first.
	child2 := stack.Expand(func() {
		fmt.Printf("Expand: %v\n", ctx.Constraints)
		layoutWidget(ctx, 10, 10)
	})

	stack.Layout(child1, child2)

	// Output:
	// Expand: {{50 50} {50 50}}
}

func ExampleList() {
	ops := new(ui.Ops)
	ctx := new(layout.Context)

	ctx.Constraints = layout.RigidConstraints(image.Point{X: 100, Y: 100})

	// The list is 1e6 elements, but only 5 fit the constraints.
	const listLen = 1e6

	var list layout.List
	count := 0
	list.Layout(cfg, q, ops, ctx, listLen, func(i int) {
		count++
		layoutWidget(ctx, 20, 20)
	})

	fmt.Println(count)

	// Output:
	// 5
}

func layoutWidget(ctx *layout.Context, width, height int) {
	ctx.Dimensions = layout.Dimensions{
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
