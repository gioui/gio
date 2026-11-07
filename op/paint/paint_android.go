package paint

// AndroidView creates a EmbedOp to render a Android view.
//
// The provided View will be resized and moved to match the clip area using:
//
// View.setLayoutParams
// View.setX
// View.setY
// View.setVisibility
//
// Additionally, Gio can remove and add the View to the ViewGroup
// to reorder the views z-index. That only happens if multiples EmbedOps
// are used in the same frame. Note that the View/SurfaceControl needs to be
// in the same ViewGroup as the View that is used to render the Gio canvas.
//
// If you need to react to some events, you need to override the appropriate
// methods of the View class, in Java/Kotlin.
//
// The View/SurfaceControl needs to be created and managed by the user, it
// must be valid for the duration of the EmbedOp (use NewGlobalRef to create
// a global reference to the View/SurfaceControl).
//
// Currently, SurfaceControl is not used, but it may be used in the future.
type AndroidView struct {
	// View is a global reference to an android.view.View.
	View uintptr
	// SurfaceControl is a global reference to an android.view.SurfaceControl.
	SurfaceControl uintptr
}

func (v AndroidView) Op() EmbedOp { return EmbedOp{view: v.View, surface: v.SurfaceControl} }
