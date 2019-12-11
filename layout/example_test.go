package layout_test

import (
	"fmt"
	"image"

	"gioui.org/layout"
	"gioui.org/unit"
)

func ExampleInset() {
	gtx := new(layout.Context)
	gtx.Reset(nil, image.Point{X: 100, Y: 100})
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
	gtx := new(layout.Context)
	// Rigid constraints with both minimum and maximum set.
	gtx.Reset(nil, image.Point{X: 100, Y: 100})

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
	gtx := new(layout.Context)
	gtx.Reset(nil, image.Point{X: 100, Y: 100})

	layout.Flex{}.Layout(gtx,
		// Rigid 10x10 widget.
		layout.Rigid(func() {
			fmt.Printf("Rigid: %v\n", gtx.Constraints.Width)
			layoutWidget(gtx, 10, 10)
		}),
		// Child with 50% space allowance.
		layout.Flexed(0.5, func() {
			fmt.Printf("50%%: %v\n", gtx.Constraints.Width)
			layoutWidget(gtx, 10, 10)
		}),
	)

	// Output:
	// Rigid: {0 100}
	// 50%: {45 45}
}

func ExampleStack() {
	gtx := new(layout.Context)
	gtx.Reset(nil, image.Point{X: 100, Y: 100})
	gtx.Constraints.Width.Min = 0
	gtx.Constraints.Height.Min = 0

	layout.Stack{}.Layout(gtx,
		// Force widget to the same size as the second.
		layout.Expanded(func() {
			fmt.Printf("Expand: %v\n", gtx.Constraints)
			layoutWidget(gtx, 10, 10)
		}),
		// Rigid 50x50 widget.
		layout.Stacked(func() {
			layoutWidget(gtx, 50, 50)
		}),
	)

	// Output:
	// Expand: {{50 100} {50 100}}
}

func ExampleList() {
	gtx := new(layout.Context)
	gtx.Reset(nil, image.Point{X: 100, Y: 100})

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
