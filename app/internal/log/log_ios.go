// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,ios

package log

/*
#cgo CFLAGS: -Werror -fmodules -fobjc-arc -x objective-c

#include "log_ios.h"
*/
import "C"

import (
	"bufio"
	"io"
	"log"
	"unsafe"
)

func init() {
	// macOS Console already includes timstamps.
	log.SetFlags(log.Flags() &^ log.LstdFlags)
	log.SetOutput(newNSLogWriter())
}

func newNSLogWriter() io.Writer {
	r, w := io.Pipe()
	go func() {
		// 1024 is an arbitrary truncation limit, taken from Android's
		// log buffer size.
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
			C.nslog(cbuf)
		}
	}()
	return w
}
