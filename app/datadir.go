// SPDX-License-Identifier: Unlicense OR MIT

//go:build !android && !gio_ohos && !openharmony

package app

import "os"

func dataDir() (string, error) {
	return os.UserConfigDir()
}
