// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony || gio_ohos_test

package app

import (
	"fmt"
	"os"
	"sync"
)

// ohosDebugEnabled reports whether verbose OHOS diagnostics are enabled.
// It is intentionally cheap to call from cold paths only; hot input/frame
// paths must never log.
func ohosDebugEnabled() bool {
	switch os.Getenv("GIO_OHOS_DEBUG") {
	case "", "0", "false", "no":
		return false
	default:
		return true
	}
}

var ohosDebugState = struct {
	mu   sync.Mutex
	seen map[string]bool
}{seen: make(map[string]bool)}

// ohosWarnf reports a degraded capability exactly once per key. Warnings are
// always visible: they describe a real loss of functionality, not noise.
func ohosWarnf(key, format string, args ...any) {
	if !ohosMarkOnce(key) {
		return
	}
	fmt.Fprintf(os.Stderr, "[gio-ohos] "+format+"\n", args...)
}

// ohosDebugf is enabled by GIO_OHOS_DEBUG. It is deduplicated by key so a
// broken platform callback cannot flood stderr.
func ohosDebugf(key, format string, args ...any) {
	if !ohosDebugEnabled() || !ohosMarkOnce(key) {
		return
	}
	fmt.Fprintf(os.Stderr, "[gio-ohos] "+format+"\n", args...)
}

func ohosMarkOnce(key string) bool {
	ohosDebugState.mu.Lock()
	defer ohosDebugState.mu.Unlock()
	if ohosDebugState.seen[key] {
		return false
	}
	ohosDebugState.seen[key] = true
	return true
}
