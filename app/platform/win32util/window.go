//go:build windows

package win32util

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/ddkwork/golibrary/mylog"
	"github.com/samber/lo"
	"golang.org/x/sys/windows"
)

func LoadIconWithResourceID(instance HINSTANCE, res uintptr) HICON {
	ret, _ := mylog.Check3(procLoadIcon.Call(
		uintptr(instance),
		res))

	return HICON(ret)
}

func LoadCursorWithResourceID(instance HINSTANCE, res uintptr) HCURSOR {
	ret, _ := mylog.Check3(procLoadCursor.Call(
		uintptr(instance),
		res))

	return HCURSOR(ret)
}

func RegisterClassEx(wndClassEx *WNDCLASSEX) ATOM {
	ret, _ := mylog.Check3(procRegisterClassEx.Call(uintptr(unsafe.Pointer(wndClassEx))))
	return ATOM(ret)
}

func RegisterClass(className string, wndproc uintptr, instance HINSTANCE) error {
	classNamePtr := mylog.Check2(syscall.UTF16PtrFromString(className))

	icon := LoadIconWithResourceID(instance, IDI_APPLICATION)

	var wc WNDCLASSEX
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.Style = CS_HREDRAW | CS_VREDRAW
	wc.LpfnWndProc = wndproc
	wc.HInstance = instance
	wc.HbrBackground = COLOR_WINDOW + 1
	wc.HIcon = icon
	wc.HCursor = LoadCursorWithResourceID(0, IDC_ARROW)
	wc.LpszClassName = classNamePtr
	wc.LpszMenuName = nil
	wc.HIconSm = icon

	if ret := RegisterClassEx(&wc); ret == 0 {
		return syscall.GetLastError()
	}

	return nil
}

func CreateWindow(className string, instance HINSTANCE, parent HWND, exStyle, style uint) HWND {
	classNamePtr := lo.Must(syscall.UTF16PtrFromString(className))

	result := CreateWindowEx(
		exStyle,
		classNamePtr,
		nil,
		style,
		CW_USEDEFAULT,
		CW_USEDEFAULT,
		CW_USEDEFAULT,
		CW_USEDEFAULT,
		parent,
		0,
		instance,
		nil)

	if result == 0 {
		errStr := fmt.Sprintf("Error occurred in CreateWindow(%s, %v, %d, %d)", className, parent, exStyle, style)
		panic(errStr)
	}

	return result
}

func CreateWindowEx(exStyle uint, className, windowName *uint16,
	style uint, x, y, width, height int, parent HWND, menu HMENU,
	instance HINSTANCE, param unsafe.Pointer,
) HWND {
	ret, _ := mylog.Check3(procCreateWindowEx.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		uintptr(style),
		uintptr(x),
		uintptr(y),
		uintptr(width),
		uintptr(height),
		uintptr(parent),
		uintptr(menu),
		uintptr(instance),
		uintptr(param)))

	return HWND(ret)
}

func MustStringToUTF16Ptr(input string) *uint16 {
	ret := mylog.Check2(syscall.UTF16PtrFromString(input))

	return ret
}

func MustStringToUTF16uintptr(input string) uintptr {
	ret := mylog.Check2(syscall.UTF16PtrFromString(input))

	return uintptr(unsafe.Pointer(ret))
}

func MustUTF16FromString(input string) []uint16 {
	ret := mylog.Check2(syscall.UTF16FromString(input))

	return ret
}

func UTF16PtrToString(input uintptr) string {
	return windows.UTF16PtrToString((*uint16)(unsafe.Pointer(input)))
}

func SetForegroundWindow(wnd HWND) bool {
	ret, _ := mylog.Check3(procSetForegroundWindow.Call(
		uintptr(wnd),
	))
	return ret != 0
}
