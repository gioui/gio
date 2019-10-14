// SPDX-License-Identifier: Unlicense OR MIT

package log

/*
#cgo LDFLAGS: -llog

#include <stdlib.h>
#include <android/log.h>
*/
import "C"

import (
	"bufio"
	"log"
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

func init() {
	// Android's logcat already includes timestamps.
	log.SetFlags(log.Flags() &^ log.LstdFlags)
	logFd(os.Stdout.Fd())
	logFd(os.Stderr.Fd())
}

func logFd(fd uintptr) {
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	if err := syscall.Dup3(int(w.Fd()), int(fd), syscall.O_CLOEXEC); err != nil {
		panic(err)
	}
	go func() {
		tag := C.CString("gio")
		defer C.free(unsafe.Pointer(tag))
		// 1024 is the truncation limit from android/log.h, plus a \n.
		lineBuf := bufio.NewReaderSize(r, 1024)
		// The buffer to pass to C, including the terminating '\0'.
		buf := make([]byte, lineBuf.Size()+1)
		cbuf := (*C.char)(unsafe.Pointer(&buf[0]))
		for {
			line, _, err := lineBuf.ReadLine()
			if err != nil {
				break
			}
			copy(buf, line)
			buf[len(line)] = 0
			C.__android_log_write(C.ANDROID_LOG_INFO, tag, cbuf)
		}
		// The garbage collector doesn't know that w's fd was dup'ed.
		// Avoid finalizing w, and thereby avoid its finalizer closing its fd.
		runtime.KeepAlive(w)
	}()
}
