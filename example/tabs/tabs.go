// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"fmt"
	"image"
	"image/color"
	"log"

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

type Tabs struct {
	list     layout.List
	tabs     []Tab
	selected int
}

type Tab struct {
	btn   widget.Button
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
					tabs.selected = tabIdx
				}
				var tabWidth int
				layout.Stack{Alignment: layout.S}.Layout(gtx,
					layout.Stacked(func() {
						tabBtn := material.Button(th, t.Title)
						tabBtn.Background = color.RGBA{}   // No background.
						tabBtn.CornerRadius = unit.Value{} // No corners.
						tabBtn.Color = color.RGBA{A: 0xff} // Black text.
						tabBtn.TextSize = unit.Sp(20)
						tabBtn.Layout(gtx, &t.btn)
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
			layout.Center.Layout(gtx, func() {
				material.H1(th, fmt.Sprintf("Tab content #%d", tabs.selected)).Layout(gtx)
			})
		}),
	)
}
