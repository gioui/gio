package layout_test

import (
	"fmt"
	"image"

	"gioui.org/layout"
	"gioui.org/op"
)

func ExampleInset() {
	gtx := layout.Context{
		Ops: new(op.Ops),
		// Loose constraints with no minimal size.
		Constraints: layout.Constraints{
			Max: image.Point{X: 100, Y: 100},
		},
	}

	// Inset all edges by 10.
	inset := layout.UniformInset(10)
	dims := inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		// Lay out a 50x50 sized widget.
		dims := layoutWidget(gtx, 50, 50)
		fmt.Println(dims.Size)
		return dims
	})

	fmt.Println(dims.Size)

	// Output:
	// (50,50)
	// (70,70)
}

func ExampleDirection() {
	gtx := layout.Context{
		Ops: new(op.Ops),
		// Rigid constraints with both minimum and maximum set.
		Constraints: layout.Exact(image.Point{X: 100, Y: 100}),
	}

	dims := layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		// Lay out a 50x50 sized widget.
		dims := layoutWidget(gtx, 50, 50)
		fmt.Println(dims.Size)
		return dims
	})

	fmt.Println(dims.Size)

	// Output:
	// (50,50)
	// (100,100)
}

func ExampleFlex() {
	gtx := layout.Context{
		Ops: new(op.Ops),
		// Rigid constraints with both minimum and maximum set.
		Constraints: layout.Exact(image.Point{X: 100, Y: 100}),
	}

	layout.Flex{WeightSum: 2}.Layout(gtx,
		// Rigid 10x10 widget.
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			fmt.Printf("Rigid: %v\n", gtx.Constraints)
			return layoutWidget(gtx, 10, 10)
		}),
		// Child with 50% space allowance.
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			fmt.Printf("50%%: %v\n", gtx.Constraints)
			return layoutWidget(gtx, 10, 10)
		}),
	)

	// Output:
	// Rigid: {(0,100) (100,100)}
	// 50%: {(45,100) (45,100)}
}

func ExampleStack() {
	gtx := layout.Context{
		Ops: new(op.Ops),
		Constraints: layout.Constraints{
			Max: image.Point{X: 100, Y: 100},
		},
	}

	layout.Stack{}.Layout(gtx,
		// Force widget to the same size as the second.
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			fmt.Printf("Expand: %v\n", gtx.Constraints)
			return layoutWidget(gtx, 10, 10)
		}),
		// Rigid 50x50 widget.
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layoutWidget(gtx, 50, 50)
		}),
	)

	// Output:
	// Expand: {(50,50) (100,100)}
}

func ExampleBackground() {
	gtx := layout.Context{
		Ops: new(op.Ops),
		Constraints: layout.Constraints{
			Max: image.Point{X: 100, Y: 100},
		},
	}

	layout.Background{}.Layout(gtx,
		// Force widget to the same size as the second.
		func(gtx layout.Context) layout.Dimensions {
			fmt.Printf("Expand: %v\n", gtx.Constraints)
			return layoutWidget(gtx, 10, 10)
		},
		// Rigid 50x50 widget.
		func(gtx layout.Context) layout.Dimensions {
			return layoutWidget(gtx, 50, 50)
		},
	)

	// Output:
	// Expand: {(50,50) (100,100)}
}

func ExampleList() {
	gtx := layout.Context{
		Ops: new(op.Ops),
		// Rigid constraints with both minimum and maximum set.
		Constraints: layout.Exact(image.Point{X: 100, Y: 100}),
	}

	// The list is 1e6 elements, but only 5 fit the constraints.
	const listLen = 1e6

	var list layout.List
	list.Layout(gtx, listLen, func(gtx layout.Context, i int) layout.Dimensions {
		return layoutWidget(gtx, 20, 20)
	})

	fmt.Println(list.Position.Count)

	// Output:
	// 5
}

func layoutWidget(ctx layout.Context, width, height int) layout.Dimensions {
	return layout.Dimensions{
		Size: image.Point{
			X: width,
			Y: height,
		},
	}
}
