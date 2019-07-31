// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"os"
	"os/signal"
	"syscall"
)

func init() {
	// Work around golang.org/issue/33384
	signal.Notify(make(chan os.Signal), syscall.SIGPIPE)
}
