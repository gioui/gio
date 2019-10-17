package layout_test

import (
	"fmt"
	"image"
	"time"

	"gioui.org/io/event"
	"gioui.org/layout"
	"gioui.org/unit"
)

type queue struct{}

type config struct{}

var q queue
var cfg = new(config)

func ExampleInset() {
	gtx := &layout.Context{Queue: q}
	gtx.Reset(cfg, image.Point{X: 100, Y: 100})
	// Loose constraints with no minimal size.
	gtx.Constraints.Width.Min = 0
	gtx.Constraints.Height.Min = 0

	// Inset all edges by 10.
	inset := layout.UniformInset(unit.Dp(10))
	inset.Layout(gtx, func() {
		// Lay out a 50x50 sized widget.
		layoutWidget(gtx, 50, 50)
		fmt.Println(gtx.Dimensions.Size)
	})

	fmt.Println(gtx.Dimensions.Size)

	// Output:
	// (50,50)
	// (70,70)
}

func ExampleAlign() {
	gtx := &layout.Context{Queue: q}
	// Rigid constraints with both minimum and maximum set.
	gtx.Reset(cfg, image.Point{X: 100, Y: 100})

	align := layout.Align(layout.Center)
	align.Layout(gtx, func() {
		// Lay out a 50x50 sized widget.
		layoutWidget(gtx, 50, 50)
		fmt.Println(gtx.Dimensions.Size)
	})

	fmt.Println(gtx.Dimensions.Size)

	// Output:
	// (50,50)
	// (100,100)
}

func ExampleFlex() {
	gtx := &layout.Context{Queue: q}
	gtx.Reset(cfg, image.Point{X: 100, Y: 100})

	flex := layout.Flex{}

	// Rigid 10x10 widget.
	child1 := flex.Rigid(gtx, func() {
		fmt.Printf("Rigid: %v\n", gtx.Constraints.Width)
		layoutWidget(gtx, 10, 10)
	})

	// Child with 50% space allowance.
	child2 := flex.Flex(gtx, 0.5, func() {
		fmt.Printf("50%%: %v\n", gtx.Constraints.Width)
		layoutWidget(gtx, 10, 10)
	})

	flex.Layout(gtx, child1, child2)

	// Output:
	// Rigid: {0 100}
	// 50%: {45 45}
}

func ExampleStack() {
	gtx := &layout.Context{Queue: q}
	gtx.Reset(cfg, image.Point{X: 100, Y: 100})

	stack := layout.Stack{}

	// Rigid 50x50 widget.
	child1 := stack.Rigid(gtx, func() {
		layoutWidget(gtx, 50, 50)
	})

	// Force widget to the same size as the first.
	child2 := stack.Expand(gtx, func() {
		fmt.Printf("Expand: %v\n", gtx.Constraints)
		layoutWidget(gtx, 10, 10)
	})

	stack.Layout(gtx, child1, child2)

	// Output:
	// Expand: {{50 100} {50 100}}
}

func ExampleList() {
	gtx := &layout.Context{Queue: q}
	gtx.Reset(cfg, image.Point{X: 100, Y: 100})

	// The list is 1e6 elements, but only 5 fit the constraints.
	const listLen = 1e6

	var list layout.List
	count := 0
	list.Layout(gtx, listLen, func(i int) {
		count++
		layoutWidget(gtx, 20, 20)
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

func (config) Px(v unit.Value) int {
	return int(v.V + .5)
}

func (queue) Events(k event.Key) []event.Event {
	return nil
}
