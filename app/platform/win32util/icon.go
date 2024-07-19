//go:build windows

package win32util

import (
	"unsafe"

	"github.com/ddkwork/golibrary/mylog"
)

func CreateIconFromResourceEx(presbits uintptr, dwResSize uint32, isIcon bool, version uint32, cxDesired int, cyDesired int, flags uint) (uintptr, error) {
	icon := 0
	if isIcon {
		icon = 1
	}
	r, _ := mylog.Check3(procCreateIconFromResourceEx.Call(
		presbits,
		uintptr(dwResSize),
		uintptr(icon),
		uintptr(version),
		uintptr(cxDesired),
		uintptr(cyDesired),
		uintptr(flags),
	))

	if r == 0 {
		return 0, nil
	}
	return r, nil
}

// CreateHIconFromPNG creates a HICON from a PNG file
func CreateHIconFromPNG(pngData []byte) (HICON, error) {
	icon := mylog.Check2(CreateIconFromResourceEx(
		uintptr(unsafe.Pointer(&pngData[0])),
		uint32(len(pngData)),
		true,
		0x00030000,
		0,
		0,
		LR_DEFAULTSIZE))
	return HICON(icon), nil
}
