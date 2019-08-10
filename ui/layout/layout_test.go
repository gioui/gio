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

var cs = layout.Constraints{
	Width: layout.Constraint{
		Max: 100,
	},
	Height: layout.Constraint{
		Max: 100,
	},
}

func ExampleInset() {
	ops := new(ui.Ops)

	// Inset all edges by 10.
	inset := layout.UniformInset(ui.Dp(10))
	cs = inset.Begin(cfg, ops, cs)
	// Lay out a 50x50 sized widget.
	dims := layoutWidget(50, 50, cs)
	dims = inset.End(dims)

	fmt.Println(dims.Size)

	// Output: (70,70)
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
