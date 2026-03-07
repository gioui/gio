//go:build darwin && !ios

package paint

// AppKitView creates an EmbedOp for embedding an AppKit NSView on macOS.
//
// The provided NSView will be repositioned and resized to match the clip area
// using setFrame. The view will be shown/hidden and reordered in the view
// hierarchy as needed.
//
// The NSView must be added to the same superview as the Gio view. Z-order is
// managed automatically based on the order of EmbedOps in the frame.
type AppKitView struct {
	// ViewController is a pointer to an NSView (CFTypeRef/void*).
	ViewController uintptr
}

func (v AppKitView) Op() EmbedOp { return EmbedOp{view: v.ViewController} }
