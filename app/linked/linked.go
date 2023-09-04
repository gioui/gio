package linked

import "image"

type LinkedView interface {
	// Using app.ViewEvent to Parent() a native window
	Parent(interface{})
	Move(image.Rectangle)
	Show()
	Hide()
	Destroy()
}
