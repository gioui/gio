// SPDX-License-Identifier: Unlicense OR MIT

// +build !android,go1.13

package app

import "os"

func dataDir() (string, error) {
	return os.UserConfigDir()
}
