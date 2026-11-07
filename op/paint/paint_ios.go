package paint

// UIKitView creates an EmbedOp for embedding a UIKit UIView on iOS.
//
// The provided UIView will be repositioned and resized to match the clip area
// using setFrame. The view will be shown/hidden and reordered in the view
// hierarchy as needed.
//
// The UIView must be added to the same superview as the Gio view. Z-order is
// managed automatically based on the order of EmbedOps in the frame.
type UIKitView struct {
	// ViewController is a pointer to a UIView (CFTypeRef/void*).
	ViewController uintptr
}

func (v UIKitView) Op() EmbedOp { return EmbedOp{view: v.ViewController} }
