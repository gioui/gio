package layout_test

import (
	"fmt"
	"image"
	"time"

	"gioui.org/ui"
	"gioui.org/ui/layout"
)

type config struct{}

var cfg = new(config)

func ExampleInset() {
	ops := new(ui.Ops)

	// Loose constraints with no minimal size.
	var cs layout.Constraints
	cs.Width.Max = 100
	cs.Height.Max = 100

	// Inset all edges by 10.
	inset := layout.UniformInset(ui.Dp(10))
	cs = inset.Begin(cfg, ops, cs)
	// Lay out a 50x50 sized widget.
	dims := layoutWidget(50, 50, cs)
	fmt.Println(dims.Size)
	dims = inset.End(dims)

	fmt.Println(dims.Size)

	// Output:
	// (50,50)
	// (70,70)
}

func ExampleAlign() {
	ops := new(ui.Ops)

	cs := layout.RigidConstraints(image.Point{X: 100, Y: 100})

	align := layout.Align{Alignment: layout.Center}
	cs = align.Begin(ops, cs)
	// Lay out a 50x50 sized widget.
	dims := layoutWidget(50, 50, cs)
	fmt.Println(dims.Size)

	dims = align.End(dims)

	fmt.Println(dims.Size)

	// Output:
	// (50,50)
	// (100,100)
}

func layoutWidget(width, height int, cs layout.Constraints) layout.Dimens {
	return layout.Dimens{
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
