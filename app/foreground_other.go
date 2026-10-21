// SPDX-License-Identifier: Unlicense OR MIT

//go:build !android

package app

func startForeground(ForegroundOptions) error { return nil }

func stopForeground() {}
