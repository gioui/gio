// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony || gio_ohos_test

package app

import (
	"strconv"

	"gioui.org/f32"
	"gioui.org/io/key"
)

// OpenHarmony key codes (native_xcomponent_key_event.h).  Only the codes the
// backend translates are listed; os_ohos.go asserts them against the NDK
// header at compile time so a header change cannot silently break input.
const (
	ohosKeyFn           = 0
	ohosKeyBack         = 2
	ohosKey0            = 2000
	ohosKey9            = 2009
	ohosKeyA            = 2017
	ohosKeyZ            = 2042
	ohosKeyComma        = 2043
	ohosKeyPeriod       = 2044
	ohosKeyAltLeft      = 2045
	ohosKeyAltRight     = 2046
	ohosKeyShiftLeft    = 2047
	ohosKeyShiftRight   = 2048
	ohosKeyTab          = 2049
	ohosKeySpace        = 2050
	ohosKeyEnter        = 2054
	ohosKeyDel          = 2055
	ohosKeyGrave        = 2056
	ohosKeyMinus        = 2057
	ohosKeyEquals       = 2058
	ohosKeyLeftBracket  = 2059
	ohosKeyRightBracket = 2060
	ohosKeyBackslash    = 2061
	ohosKeySemicolon    = 2062
	ohosKeyApostrophe   = 2063
	ohosKeySlash        = 2064
	ohosKeyPageUp       = 2068
	ohosKeyPageDown     = 2069
	ohosKeyEscape       = 2070
	ohosKeyForwardDel   = 2071
	ohosKeyCtrlLeft     = 2072
	ohosKeyCtrlRight    = 2073
	ohosKeyMetaLeft     = 2076
	ohosKeyMetaRight    = 2077
	ohosKeyMoveHome     = 2081
	ohosKeyMoveEnd      = 2082
	ohosKeyInsert       = 2083
	ohosKeyF1           = 2090
	ohosKeyF12          = 2101
	ohosKeyNumpad0      = 2103
	ohosKeyNumpad9      = 2112
	ohosKeyNumpadEnter  = 2119
	ohosKeyDpadUp       = 2012
	ohosKeyDpadDown     = 2013
	ohosKeyDpadLeft     = 2014
	ohosKeyDpadRight    = 2015
	ohosKeyDpadCenter   = 2016
)

// OpenHarmony modifier bits (ArkUI_ModifierKeyName).
const (
	ohosModCtrl  = 1 << 0
	ohosModShift = 1 << 1
	ohosModAlt   = 1 << 2
	ohosModFn    = 1 << 3
)

// ohosPinchScroll converts a relative pinch scale (1.0 = unchanged, opening
// the fingers > 1) into a wheel delta.  Gio and gpofd both use "negative
// Scroll.Y = zoom in", so opening must produce a negative delta; we also add
// Ctrl so canvas.wheelZoom selects the zoom path, matching trackpad pinch on
// the desktop. Opening must zoom in; see docs/ohos-remediation-plan.md (WP6).
const ohosPinchScroll = 400

// ohosKeyName translates an OpenHarmony key code into a Gio key name.
func ohosKeyName(code int) (key.Name, bool) {
	switch code {
	case ohosKeyBack:
		return key.NameBack, true
	case ohosKeyForwardDel:
		return key.NameDeleteForward, true
	case ohosKeyDel:
		return key.NameDeleteBackward, true
	case ohosKeyNumpadEnter:
		return key.NameEnter, true
	case ohosKeyEnter:
		return key.NameReturn, true
	case ohosKeyCtrlLeft, ohosKeyCtrlRight:
		return key.NameCtrl, true
	case ohosKeyShiftLeft, ohosKeyShiftRight:
		return key.NameShift, true
	case ohosKeyAltLeft, ohosKeyAltRight:
		return key.NameAlt, true
	case ohosKeyMetaLeft, ohosKeyMetaRight:
		return key.NameSuper, true
	case ohosKeyDpadUp:
		return key.NameUpArrow, true
	case ohosKeyDpadDown:
		return key.NameDownArrow, true
	case ohosKeyDpadLeft:
		return key.NameLeftArrow, true
	case ohosKeyDpadRight:
		return key.NameRightArrow, true
	case ohosKeyTab:
		return key.NameTab, true
	case ohosKeyEscape:
		return key.NameEscape, true
	case ohosKeyMoveHome:
		return key.NameHome, true
	case ohosKeyMoveEnd:
		return key.NameEnd, true
	case ohosKeyPageUp:
		return key.NamePageUp, true
	case ohosKeyPageDown:
		return key.NamePageDown, true
	case ohosKeySpace:
		return key.NameSpace, true
	case ohosKeyInsert:
		return key.NameInsert, true
	}
	switch c := code; {
	case c >= ohosKeyA && c <= ohosKeyZ:
		return key.Name(rune('A' + c - ohosKeyA)), true
	case c >= ohosKey0 && c <= ohosKey9:
		return key.Name(rune('0' + c - ohosKey0)), true
	case c >= ohosKeyF1 && c <= ohosKeyF12:
		return key.Name("F" + strconv.Itoa(c-ohosKeyF1+1)), true
	case c >= ohosKeyNumpad0 && c <= ohosKeyNumpad9:
		return key.Name(rune('0' + c - ohosKeyNumpad0)), true
	}
	s, ok := ohosSymbolKeys[code]
	if !ok {
		return "", false
	}
	return s, true
}

// ohosSymbolKeys names punctuation keys by their unshifted character.
var ohosSymbolKeys = map[int]key.Name{
	ohosKeyGrave:        "`",
	ohosKeyMinus:        "-",
	ohosKeyEquals:       "=",
	ohosKeyLeftBracket:  "[",
	ohosKeyRightBracket: "]",
	ohosKeyBackslash:    "\\",
	ohosKeySemicolon:    ";",
	ohosKeyApostrophe:   "'",
	ohosKeyComma:        ",",
	ohosKeyPeriod:       ".",
	ohosKeySlash:        "/",
}

var ohosShiftedSymbols = map[int]string{
	ohosKeyGrave:        "~",
	ohosKeyMinus:        "_",
	ohosKeyEquals:       "+",
	ohosKeyLeftBracket:  "{",
	ohosKeyRightBracket: "}",
	ohosKeyBackslash:    "|",
	ohosKeySemicolon:    ":",
	ohosKeyApostrophe:   "\"",
	ohosKeyComma:        "<",
	ohosKeyPeriod:       ">",
	ohosKeySlash:        "?",
}

// ohosModifiers translates the OpenHarmony modifier bit mask.  Fn is not a
// Gio modifier and is ignored; Meta is reported as Super.
func ohosModifiers(mods int) key.Modifiers {
	var m key.Modifiers
	if mods&ohosModCtrl != 0 {
		m |= key.ModCtrl
	}
	if mods&ohosModShift != 0 {
		m |= key.ModShift
	}
	if mods&ohosModAlt != 0 {
		m |= key.ModAlt
	}
	return m
}

// ohosPrintable returns the text a printable key press inserts.  Chords with
// Ctrl, Alt or Super are shortcuts, not text (same rule as the Android and
// Wayland backends).  Shift is applied for letters, digits and punctuation.
func ohosPrintable(code int, mods int) (string, bool) {
	if mods&(ohosModCtrl|ohosModAlt) != 0 {
		return "", false
	}
	shift := mods&ohosModShift != 0
	switch c := code; {
	case c >= ohosKeyA && c <= ohosKeyZ:
		r := rune('a' + c - ohosKeyA)
		if shift {
			r = rune('A' + c - ohosKeyA)
		}
		return string(r), true
	case c >= ohosKey0 && c <= ohosKey9:
		if shift {
			return string(")!@#$%^&*("[c-ohosKey0]), true
		}
		return string(rune('0' + c - ohosKey0)), true
	case c >= ohosKeyNumpad0 && c <= ohosKeyNumpad9:
		return string(rune('0' + c - ohosKeyNumpad0)), true
	case c == ohosKeySpace:
		return " ", true
	}
	if shift {
		if s, ok := ohosShiftedSymbols[code]; ok {
			return s, true
		}
	}
	if s, ok := ohosSymbolKeys[code]; ok {
		return string(s), true
	}
	return "", false
}

// ohosAxisScroll converts axis values into a pointer scroll delta.  vertical
// and horizontal are device scroll amounts (positive = down/right); pinch is
// the relative scale (1.0 = unchanged, > 1 = opening).  A pinch also sets the
// Ctrl modifier so the canvas takes its zoom path.
func ohosAxisScroll(vertical, horizontal, pinch float64, mods int) (f32.Point, key.Modifiers) {
	m := ohosModifiers(mods)
	dx := float32(horizontal)
	dy := float32(vertical)
	if pinch != 0 && pinch != 1 {
		dy += float32(-(pinch - 1) * ohosPinchScroll)
		m |= key.ModCtrl
	}
	return f32.Pt(dx, dy), m
}

// ohosAxisChanged reports whether an axis event carries a usable delta.
func ohosAxisChanged(vertical, horizontal, pinch float64) bool {
	return vertical != 0 || horizontal != 0 || (pinch != 0 && pinch != 1)
}
