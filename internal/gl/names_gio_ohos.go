// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony

package gl

// HarmonyOS ships libGLESv3.so/libGLESv2.so without the versioned .so.3.0
// names desktop Linux uses (docs/44 §4.3).
func init() {
	ohosLibNames = []string{"libGLESv3.so", "libGLESv2.so"}
}
