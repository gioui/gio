// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"gioui.org/font/gofont"
)

func main() {
	go func() {
		w := app.NewWindow()
		if err := loop(w); err != nil {
			log.Fatal(err)
		}
	}()
	app.Main()
}

func loop(w *app.Window) error {
	gofont.Register()
	th := material.NewTheme()
	gtx := new(layout.Context)
	for {
		e := <-w.Events()
		switch e := e.(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			gtx.Reset(e.Queue, e.Config, e.Size)
			drawTabs(gtx, th)
			e.Frame(gtx.Ops)
		}
	}
}

var tabs Tabs
var slider Slider

type Tabs struct {
	list     layout.List
	tabs     []Tab
	selected int
}

type Tab struct {
	btn   widget.Clickable
	Title string
}

func init() {
	for i := 1; i <= 100; i++ {
		tabs.tabs = append(tabs.tabs,
			Tab{Title: fmt.Sprintf("Tab %d", i)},
		)
	}
}

func drawTabs(gtx *layout.Context, th *material.Theme) {
	layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func() {
			tabs.list.Layout(gtx, len(tabs.tabs), func(tabIdx int) {
				t := &tabs.tabs[tabIdx]
				if t.btn.Clicked(gtx) {
					if tabs.selected < tabIdx {
						slider.PushLeft(gtx)
					} else if tabs.selected > tabIdx {
						slider.PushRight(gtx)
					}
					tabs.selected = tabIdx
				}
				var tabWidth int
				layout.Stack{Alignment: layout.S}.Layout(gtx,
					layout.Stacked(func() {
						material.Clickable(gtx, &t.btn, func() {
							layout.UniformInset(unit.Sp(12)).Layout(gtx, func() {
								material.H6(th, t.Title).Layout(gtx)
							})
						})
						tabWidth = gtx.Dimensions.Size.X
					}),
					layout.Stacked(func() {
						if tabs.selected != tabIdx {
							return
						}
						paint.ColorOp{Color: th.Color.Primary}.Add(gtx.Ops)
						tabHeight := gtx.Px(unit.Dp(4))
						paint.PaintOp{Rect: f32.Rectangle{
							Max: f32.Point{
								X: float32(tabWidth),
								Y: float32(tabHeight),
							},
						}}.Add(gtx.Ops)
						gtx.Dimensions = layout.Dimensions{
							Size: image.Point{X: tabWidth, Y: tabHeight},
						}
					}),
				)
			})
		}),
		layout.Flexed(1, func() {
			slider.Layout(gtx, func() {
				fill(gtx, dynamicColor(tabs.selected))
				layout.Center.Layout(gtx, func() {
					material.H1(th, fmt.Sprintf("Tab content #%d", tabs.selected+1)).Layout(gtx)
				})
			})
		}),
	)
}

func bounds(gtx *layout.Context) f32.Rectangle {
	cs := gtx.Constraints
	d := image.Point{X: cs.Width.Min, Y: cs.Height.Min}
	return f32.Rectangle{
		Max: f32.Point{X: float32(d.X), Y: float32(d.Y)},
	}
}

func fill(gtx *layout.Context, col color.RGBA) {
	dr := bounds(gtx)
	paint.ColorOp{Color: col}.Add(gtx.Ops)
	paint.PaintOp{Rect: dr}.Add(gtx.Ops)
}

func dynamicColor(i int) color.RGBA {
	sn, cs := math.Sincos(float64(i) * math.Phi)
	return color.RGBA{
		R: 0xA0 + byte(0x30*sn),
		G: 0xA0 + byte(0x30*cs),
		B: 0xD0,
		A: 0xFF,
	}
}
