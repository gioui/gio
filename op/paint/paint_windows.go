package paint

// Win32View creates an EmbedOp for embedding a Win32 window (HWND).
//
// The provided HWND will be repositioned and resized to match the clip area
// using SetWindowPos. The window will be shown/hidden as needed.
//
// The HWND must be a child window of the Gio window, created with WS_CHILD style.
// Z-order is managed automatically when multiple EmbedOps are used.
type Win32View struct {
	// HWND is a handle to a Win32 window (child window).
	HWND uintptr
}

func (v Win32View) Op() EmbedOp { return EmbedOp{view: v.HWND} }
