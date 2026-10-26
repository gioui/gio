// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony

package app

import "errors"

// dataDir returns the sandbox directory injected by gio_ohos_configure.
// os.UserConfigDir is unusable on HarmonyOS because the sandbox has no HOME
// or XDG_* until the shim supplies them.
func dataDir() (string, error) {
	ohosState.mu.Lock()
	dir := ohosState.dataDir
	ohosState.mu.Unlock()
	if dir == "" {
		return "", errors.New("app: data directory is not configured (gio_ohos_configure was not called)")
	}
	return dir, nil
}
