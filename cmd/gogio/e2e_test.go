// SPDX-License-Identifier: Unlicense OR MIT

package main_test

import (
	"flag"
	"image"
	"image/color"
	"testing"
)

var raceEnabled = false

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
	// platform, with its initial frame fully drawn.
	Start(t *testing.T, path string, width, height int)

	// Screenshot takes a screenshot of the Gio app on the platform.
	Screenshot() image.Image

	// Click performs a pointer click at the specified coordinates,
	// including both press and release. It returns when the next frame is
	// fully drawn.
	Click(x, y int)
}

func TestEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skipf("end-to-end tests tend to be slow")
	}

	t.Parallel()

	// Keep this list local, to not reuse TestDriver objects.
	subtests := []struct {
		name   string
		driver TestDriver
	}{
		{"X11", &X11TestDriver{}},
		{"Wayland", &WaylandTestDriver{}},
		{"JS", &JSTestDriver{}},
	}

	for _, subtest := range subtests {
		t.Run(subtest.name, func(t *testing.T) {
			subtest := subtest // copy the changing loop variable
			t.Parallel()
			runEndToEndTest(t, subtest.driver)
		})
	}
}

func runEndToEndTest(t *testing.T, driver TestDriver) {
	size := image.Point{X: 800, Y: 600}
	driver.Start(t, "testdata/red.go", size.X, size.Y)

	// The colors are split in four rectangular sections. Check the corners
	// of each of the sections. We check the corners left to right, top to
	// bottom, like when reading left-to-right text.
	wantColors := func(topLeft, topRight, botLeft, botRight color.RGBA) {
		t.Helper()
		img := driver.Screenshot()
		size_ := img.Bounds().Size()
		if size_ != size {
			if !*headless {
				// Some non-headless drivers, like Sway, may get
				// their window resized by the host window manager.
				// Run the rest of the test with the new size.
				size = size_
			} else {
				t.Fatalf("expected dimensions to be %v, got %v",
					size, size_)
			}
		}
		{
			minX, minY := 5, 5
			maxX, maxY := (size.X/2)-5, (size.Y/2)-5
			wantColor(t, img, minX, minY, topLeft)
			wantColor(t, img, maxX, minY, topLeft)
			wantColor(t, img, minX, maxY, topLeft)
			wantColor(t, img, maxX, maxY, topLeft)
		}
		{
			minX, minY := (size.X/2)+5, 5
			maxX, maxY := size.X-5, (size.Y/2)-5
			wantColor(t, img, minX, minY, topRight)
			wantColor(t, img, maxX, minY, topRight)
			wantColor(t, img, minX, maxY, topRight)
			wantColor(t, img, maxX, maxY, topRight)
		}
		{
			minX, minY := 5, (size.Y/2)+5
			maxX, maxY := (size.X/2)-5, size.Y-5
			wantColor(t, img, minX, minY, botLeft)
			wantColor(t, img, maxX, minY, botLeft)
			wantColor(t, img, minX, maxY, botLeft)
			wantColor(t, img, maxX, maxY, botLeft)
		}
		{
			minX, minY := (size.X/2)+5, (size.Y/2)+5
			maxX, maxY := size.X-5, size.Y-5
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
	// TODO(mvdan): implement this properly in the Wayland driver; swaymsg
	// almost works to automate clicks, but the button presses end up in the
	// wrong coordinates.
	if _, ok := driver.(*WaylandTestDriver); ok {
		return
	}
	driver.Click(1*(size.X/4), 1*(size.Y/4))
	driver.Click(3*(size.X/4), 3*(size.Y/4))
	wantColors(red, white, black, red)
}

func wantColor(t *testing.T, img image.Image, x, y int, want color.Color) {
	t.Helper()
	r, g, b, _ := want.RGBA()
	got := img.At(x, y)
	r_, g_, b_, _ := got.RGBA()
	if r_ != r || g_ != g || b_ != b {
		t.Errorf("got 0x%04x%04x%04x at (%d,%d), want 0x%04x%04x%04x",
			r_, g_, b_, x, y, r, g, b)
	}
}
