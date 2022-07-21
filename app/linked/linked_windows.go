package linked

import (
	"image"

	"gioui.org/app/internal/windows"

	syscall "golang.org/x/sys/windows"
)

type Link struct {
	Handle syscall.Handle
}

func New(v uintptr) Link {
	return Link{Handle: syscall.Handle(v)}
}

func (W Link) Parent(P interface{}) {
	windows.SetParent(W.Handle, syscall.Handle(P.(uintptr)))
}

func (W Link) Move(r image.Rectangle) {
	windows.MoveWindow(W.Handle, int32(r.Min.X), int32(r.Min.Y), int32(r.Dx()), int32(r.Dy()), false)
}

func (W Link) Show() {
	windows.ShowWindow(W.Handle, 1)
}

func (W Link) Hide() {
	windows.ShowWindow(W.Handle, 0)
}

func (W Link) Destroy() {
	windows.CloseWindow(W.Handle)
}
