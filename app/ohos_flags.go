// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony || gio_ohos_test

package app

import "os"

func ohosEnvFlag(name string) bool {
	switch os.Getenv(name) {
	case "", "0", "false", "no":
		return false
	default:
		return true
	}
}

// ohosSyncFrameEnabled forces a frame immediately instead of waiting for a
// vsync callback. It is a diagnostic fallback for a shim whose invalidate
// implementation is broken.
func ohosSyncFrameEnabled() bool {
	return ohosEnvFlag("GIO_OHOS_SYNC_FRAME")
}

// ohosA11yEnabled can disable semantic publishing for diagnosis.
func ohosA11yEnabled() bool {
	return !ohosEnvFlag("GIO_OHOS_NO_A11Y")
}
