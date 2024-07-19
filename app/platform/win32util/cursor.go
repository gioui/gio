//go:build windows

package win32util

import (
	"unsafe"

	"github.com/ddkwork/golibrary/mylog"
)

func GetCursorPos() (x, y int, ok bool) {
	pt := POINT{}
	ret, _ := mylog.Check3(procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt))))
	return int(pt.X), int(pt.Y), ret != 0
}
