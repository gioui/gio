// SPDX-License-Identifier: Unlicense OR MIT

package layout

import "image"

// ScreenRectCache stores screen-space rectangles for a set of named items,
// recomputed each frame from world-space coordinates and camera state.
// It provides viewport culling and hit-testing — common operations for any
// interactive canvas application (image editors, map viewers, diagram tools).
//
// Usage:
//
//	var cache layout.ScreenRectCache
//
//	// Each frame, after computing screen rects:
//	cache.BeginFrame()
//	for i, worldRect := range worldRects {
//	    screenX, screenY := worldToScreen(worldRect.Min)
//	    screenW, screenH := worldToScreenSize(worldRect.Size())
//	    cache.Set(i, image.Rect(screenX, screenY, screenX+screenW, screenY+screenH))
//	}
//	cache.EndFrame(viewport) // culls rects fully outside viewport
//
//	// For hit-testing:
//	if idx, ok := cache.HitTest(cursorX, cursorY); ok {
//	    selectZone(idx)
//	}
type ScreenRectCache struct {
	rects    []image.Rectangle
	viewport image.Rectangle
}

// BeginFrame resets the cache for a new frame. Call before Set.
func (c *ScreenRectCache) BeginFrame() {
	c.rects = c.rects[:0]
}

// Set records the screen-space rectangle for item at index idx.
// Rects are appended in order — idx must be sequential starting from 0.
func (c *ScreenRectCache) Set(idx int, r image.Rectangle) {
	for len(c.rects) <= idx {
		c.rects = append(c.rects, image.Rectangle{})
	}
	c.rects[idx] = r
}

// EndFrame culls rects fully outside the viewport.
// Items outside the viewport get zero-size rects (they still exist for
// index stability, but draw/hit-test functions skip zero-size rects).
func (c *ScreenRectCache) EndFrame(viewport image.Rectangle) {
	c.viewport = viewport
	for i := range c.rects {
		r := &c.rects[i]
		if r.Max.X < viewport.Min.X || r.Min.X > viewport.Max.X ||
			r.Max.Y < viewport.Min.Y || r.Min.Y > viewport.Max.Y {
			*r = image.Rectangle{} // cull
		}
	}
}

// Get returns the screen-space rectangle for item idx.
// Returns a zero rect if idx is out of range.
func (c *ScreenRectCache) Get(idx int) image.Rectangle {
	if idx < 0 || idx >= len(c.rects) {
		return image.Rectangle{}
	}
	return c.rects[idx]
}

// Len returns the number of items in the cache.
func (c *ScreenRectCache) Len() int {
	return len(c.rects)
}

// HitTest finds the last (highest-priority) item whose rect contains (x, y).
// Returns the item index and whether a match was found.
// Items are tested in reverse order — later items win (like Gio's tag system).
func (c *ScreenRectCache) HitTest(x, y int) (idx int, found bool) {
	for i := len(c.rects) - 1; i >= 0; i-- {
		r := c.rects[i]
		if r.Dx() <= 0 || r.Dy() <= 0 {
			continue // culled or empty
		}
		if x >= r.Min.X && x < r.Max.X && y >= r.Min.Y && y < r.Max.Y {
			return i, true
		}
	}
	return -1, false
}

// HitTestDepth finds the item with the highest depth value whose rect
// contains (x, y). Useful when items have a z-order (depth) field.
// depths must be parallel to the rects (same length).
func (c *ScreenRectCache) HitTestDepth(x, y int, depths []int) (idx int, found bool) {
	bestDepth := -1
	for i := range c.rects {
		r := c.rects[i]
		if r.Dx() <= 0 || r.Dy() <= 0 {
			continue
		}
		if x >= r.Min.X && x < r.Max.X && y >= r.Min.Y && y < r.Max.Y {
			d := 0
			if i < len(depths) {
				d = depths[i]
			}
			if d > bestDepth {
				bestDepth = d
				idx = i
				found = true
			}
		}
	}
	return
}

// ClipsTo returns true if the item at idx is fully visible within the viewport.
func (c *ScreenRectCache) ClipsTo(idx int) bool {
	r := c.Get(idx)
	return !r.Empty() && r.Min.X >= c.viewport.Min.X && r.Max.X <= c.viewport.Max.X &&
		r.Min.Y >= c.viewport.Min.Y && r.Max.Y <= c.viewport.Max.Y
}
