// SPDX-License-Identifier: Unlicense OR MIT

package main_test

import (
	"flag"
	"image"
	"image/color"
	"testing"
)

var headless = flag.Bool("headless", true, "run end-to-end tests in headless mode")

// TestDriver is implemented by each of the platforms we can run end-to-end
// tests on. None of its methods return any errors, as the errors are directly
// reported to testing.T via methods like Fatal.
type TestDriver interface {
	// Start provides the test driver with a testing.T, as well as the path
	// to the Gio app to use for the test. The app will be run with the
	// given width and height, and the platform's background should be
	// white.
	//
	// When the function returns, the gio app must be ready to use on the
	// platform.
	//
	// The returned cleanup funcs must be run in reverse order, to mimic
	// deferred funcs.
	// TODO(mvdan): replace with testing.T.Cleanup once Go 1.14 is out.
	Start(t *testing.T, path string, width, height int) (cleanups []func())

	// Screenshot takes a screenshot of the Gio app on the platform.
	Screenshot() image.Image

	// Click performs a pointer click at the specified coordinates,
	// including both press and release.
	Click(x, y int)
}

func runEndToEndTest(t *testing.T, driver TestDriver) {
	width, height := 800, 600
	cleanups := driver.Start(t, "testdata/red.go", width, height)

	// The colors are split in four rectangular sections. Check the corners
	// of each of the sections. We check the corners left to right, top to
	// bottom, like when reading left-to-right text.
	wantColors := func(topLeft, topRight, botLeft, botRight color.RGBA) {
		img := driver.Screenshot()
		size := img.Bounds().Size()
		// We expect to receive a width*height screenshot.
		if size.X != width || size.Y != height {
			t.Fatalf("expected dimensions to be %d*%d, got %d*%d",
				width, height, size.X, size.Y)
		}
		{
			minX, minY := 5, 5
			maxX, maxY := (width/2)-5, (height/2)-5
			wantColor(t, img, minX, minY, topLeft)
			wantColor(t, img, maxX, minY, topLeft)
			wantColor(t, img, minX, maxY, topLeft)
			wantColor(t, img, maxX, maxY, topLeft)
		}
		{
			minX, minY := (width/2)+5, 5
			maxX, maxY := width-5, (height/2)-5
			wantColor(t, img, minX, minY, topRight)
			wantColor(t, img, maxX, minY, topRight)
			wantColor(t, img, minX, maxY, topRight)
			wantColor(t, img, maxX, maxY, topRight)
		}
		{
			minX, minY := 5, (height/2)+5
			maxX, maxY := (width/2)-5, height-5
			wantColor(t, img, minX, minY, botLeft)
			wantColor(t, img, maxX, minY, botLeft)
			wantColor(t, img, minX, maxY, botLeft)
			wantColor(t, img, maxX, maxY, botLeft)
		}
		{
			minX, minY := (width/2)+5, (height/2)+5
			maxX, maxY := width-5, height-5
			wantColor(t, img, minX, minY, botRight)
			wantColor(t, img, maxX, minY, botRight)
			wantColor(t, img, minX, maxY, botRight)
			wantColor(t, img, maxX, maxY, botRight)
		}
	}

	beef := color.RGBA{R: 0xde, G: 0xad, B: 0xbe}
	white := color.RGBA{R: 0xff, G: 0xff, B: 0xff}
	black := color.RGBA{R: 0x00, G: 0x00, B: 0x00}
	gray := color.RGBA{R: 0xbb, G: 0xbb, B: 0xbb}
	red := color.RGBA{R: 0xff, G: 0x00, B: 0x00}

	// These are the four colors at the beginning.
	wantColors(beef, white, black, gray)

	// Click the first and last sections to turn them red.
	driver.Click(1*(width/4), 1*(height/4))
	driver.Click(3*(width/4), 3*(height/4))
	wantColors(red, white, black, red)

	// Run the cleanup funcs from last to first, as if they were defers.
	for i := len(cleanups) - 1; i >= 0; i-- {
		cleanups[i]()
	}
}

func wantColor(t *testing.T, img image.Image, x, y int, want color.Color) {
	r, g, b, _ := want.RGBA()
	got := img.At(x, y)
	r_, g_, b_, _ := got.RGBA()
	if r_ != r || g_ != g || b_ != b {
		t.Errorf("got 0x%04x%04x%04x at (%d,%d), want 0x%04x%04x%04x",
			r_, g_, b_, x, y, r, g, b)
	}
}
