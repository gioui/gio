//go:build linux

package win32util

type (
	HANDLE uintptr
	HWND   = HANDLE
)

func SetTheme(hwnd HWND, useDarkMode bool) {
	//    todo implement android dark mode support
}
