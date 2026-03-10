// SPDX-License-Identifier: Unlicense OR MIT

package key

import (
	"strings"
	"syscall/js"
)

// ModShortcut is the platform's shortcut modifier, usually the ctrl
// modifier. On Apple platforms it is the cmd key.
var ModShortcut = ModCtrl

// ModShortcut is the platform's alternative shortcut modifier,
// usually the ctrl modifier. On Apple platforms it is the alt modifier.
var ModShortcutAlt = ModCtrl

func init() {
	nav := js.Global().Get("navigator")
	if !nav.Truthy() {
		return // Almost impossible to happen
	}

	platform := ""
	if p := nav.Get("platform"); p.Truthy() {
		platform = p.String()
	}
	platform = strings.ToLower(platform)

	// Based on https://developer.mozilla.org/en-US/docs/Web/API/Navigator/platform#examples
	for _, darwinPlatform := range []string{"mac", "iphone", "ipad", "ipod"} {
		if strings.HasPrefix(platform, darwinPlatform) {
			ModShortcut = ModCommand
			ModShortcutAlt = ModAlt
			return
		}
	}
}
