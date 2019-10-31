// SPDX-License-Identifier: Unlicense OR MIT

package main_test

import (
	"flag"
	"image"
	"testing"
)

var headless = flag.Bool("headless", true, "run end-to-end tests in headless mode")

// TestDriver is implemented by each of the platforms we can run end-to-end
// tests on. None of its methods return any errors, as the errors are directly
// reported to testing.T via methods like Fatal.
type TestDriver interface {
	// Start provides the test driver with a testing.T, as well as the path
	// to the Gio app to use for the test. The app will be run with the
	// given width and height. When the function returns, the gio app must
	// be ready to use on the platform.
	//
	// The returned cleanup funcs must be run in reverse order, to mimic
	// deferred funcs.
	// TODO(mvdan): replace with testing.T.Cleanup once Go 1.14 is out.
	Start(t *testing.T, path string, width, height int) (cleanups []func())

	// Screenshot takes a screenshot of the Gio app on the platform.
	Screenshot() image.Image
}

func runEndToEndTest(t *testing.T, driver TestDriver) {
	width, height := 800, 600
	cleanups := driver.Start(t, "testdata/red.go", width, height)

	// We expect to receive a 800x600px screenshot.
	img := driver.Screenshot()
	size := img.Bounds().Size()
	if size.X != width || size.Y != height {
		t.Fatalf("expected dimensions to be %d*%d, got %d*%d",
			width, height, size.X, size.Y)
	}

	// The colors are split in four rectangular sections. Check the corners
	// of each of the sections. We check the corners left to right, top to
	// bottom, like when reading left-to-right text.

	// The top left should be 0xdeadbe.
	{
		minX, minY := 5, 5
		maxX, maxY := (width/2)-5, (height/2)-5
		wantColor(t, img, minX, minY, 0xdede, 0xadad, 0xbebe)
		wantColor(t, img, maxX, minY, 0xdede, 0xadad, 0xbebe)
		wantColor(t, img, minX, maxY, 0xdede, 0xadad, 0xbebe)
		wantColor(t, img, maxX, maxY, 0xdede, 0xadad, 0xbebe)
	}

	// The top right should be 0xffffff.
	{
		minX, minY := (width/2)+5, 5
		maxX, maxY := width-5, (height/2)-5
		wantColor(t, img, minX, minY, 0xffff, 0xffff, 0xffff)
		wantColor(t, img, maxX, minY, 0xffff, 0xffff, 0xffff)
		wantColor(t, img, minX, maxY, 0xffff, 0xffff, 0xffff)
		wantColor(t, img, maxX, maxY, 0xffff, 0xffff, 0xffff)
	}

	// The bottom left should be 0x000000.
	{
		minX, minY := 5, (height/2)+5
		maxX, maxY := (width/2)-5, height-5
		wantColor(t, img, minX, minY, 0x0000, 0x0000, 0x0000)
		wantColor(t, img, maxX, minY, 0x0000, 0x0000, 0x0000)
		wantColor(t, img, minX, maxY, 0x0000, 0x0000, 0x0000)
		wantColor(t, img, maxX, maxY, 0x0000, 0x0000, 0x0000)
	}

	// The bottom right is black (0x000000) with 0x80 alpha, so we should
	// see gray (0xbbbbbb).
	{
		minX, minY := (width/2)+5, (height/2)+5
		maxX, maxY := width-5, height-5
		wantColor(t, img, minX, minY, 0xbbbb, 0xbbbb, 0xbbbb)
		wantColor(t, img, maxX, minY, 0xbbbb, 0xbbbb, 0xbbbb)
		wantColor(t, img, minX, maxY, 0xbbbb, 0xbbbb, 0xbbbb)
		wantColor(t, img, maxX, maxY, 0xbbbb, 0xbbbb, 0xbbbb)
	}

	// Run the cleanup funcs from last to first, as if they were defers.
	for i := len(cleanups) - 1; i >= 0; i-- {
		cleanups[i]()
	}
}

func wantColor(t *testing.T, img image.Image, x, y int, r, g, b uint32) {
	color := img.At(x, y)
	r_, g_, b_, _ := color.RGBA()
	if r_ != r || g_ != g || b_ != b {
		t.Errorf("got 0x%04x%04x%04x at (%d,%d), want 0x%04x%04x%04x",
			r_, g_, b_, x, y, r, g, b)
	}
}
