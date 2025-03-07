// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"log"
	"unsafe"

	syscall "golang.org/x/sys/windows"
)

type logger struct{}

var (
	kernel32           = syscall.NewLazySystemDLL("kernel32")
	outputDebugStringW = kernel32.NewProc("OutputDebugStringW")
	debugView          *logger
)

func init() {
	// Windows DebugView already includes timestamps.
	if syscall.Stderr == 0 {
		log.SetFlags(log.Flags() &^ log.LstdFlags)
		log.SetOutput(debugView)
	}
}

func (l *logger) Write(buf []byte) (int, error) {
	p, err := syscall.UTF16PtrFromString(string(buf))
	if err != nil {
		return 0, err
	}
	outputDebugStringW.Call(uintptr(unsafe.Pointer(p)))
	return len(buf), nil
}
