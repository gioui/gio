// SPDX-License-Identifier: Unlicense OR MIT

// +build !android

package app

/*
#cgo LDFLAGS: -lxkbcommon

#include <stdlib.h>
#include <xkbcommon/xkbcommon.h>
#include <xkbcommon/xkbcommon-compose.h>
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"gioui.org/io/key"
)

type xkb struct {
	ctx       *C.struct_xkb_context
	keyMap    *C.struct_xkb_keymap
	state     *C.struct_xkb_state
	compTable *C.struct_xkb_compose_table
	compState *C.struct_xkb_compose_state
	utf8Buf   []byte
}

var (
	_XKB_MOD_NAME_CTRL  = []byte("Control\x00")
	_XKB_MOD_NAME_SHIFT = []byte("Shift\x00")
)

func (x *xkb) Destroy() {
	if x.state != nil {
		C.xkb_compose_state_unref(x.compState)
		x.compState = nil
	}
	if x.compTable != nil {
		C.xkb_compose_table_unref(x.compTable)
		x.compTable = nil
	}
	if x.state != nil {
		C.xkb_state_unref(x.state)
		x.state = nil
	}
	if x.keyMap != nil {
		C.xkb_keymap_unref(x.keyMap)
		x.keyMap = nil
	}
	if x.ctx != nil {
		C.xkb_context_unref(x.ctx)
		x.ctx = nil
	}
}

func newXKB(format C.uint32_t, fd C.int32_t, size C.uint32_t) (*xkb, error) {
	xkb := &xkb{
		ctx: C.xkb_context_new(C.XKB_CONTEXT_NO_FLAGS),
	}
	if xkb.ctx == nil {
		return nil, errors.New("newXKB: xkb_context_new failed")
	}
	locale := os.Getenv("LC_ALL")
	if locale == "" {
		locale = os.Getenv("LC_CTYPE")
	}
	if locale == "" {
		locale = os.Getenv("LANG")
	}
	if locale == "" {
		locale = "C"
	}
	cloc := C.CString(locale)
	defer C.free(unsafe.Pointer(cloc))
	xkb.compTable = C.xkb_compose_table_new_from_locale(xkb.ctx, cloc, C.XKB_COMPOSE_COMPILE_NO_FLAGS)
	if xkb.compTable == nil {
		xkb.Destroy()
		return nil, errors.New("newXKB: xkb_compose_table_new_from_locale failed")
	}
	xkb.compState = C.xkb_compose_state_new(xkb.compTable, C.XKB_COMPOSE_STATE_NO_FLAGS)
	if xkb.compState == nil {
		xkb.Destroy()
		return nil, errors.New("newXKB: xkb_compose_state_new failed")
	}
	mapData, err := syscall.Mmap(int(fd), 0, int(size), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		xkb.Destroy()
		return nil, fmt.Errorf("newXKB: mmap of keymap failed: %v", err)
	}
	defer syscall.Munmap(mapData)
	xkb.keyMap = C.xkb_keymap_new_from_buffer(xkb.ctx, (*C.char)(unsafe.Pointer(&mapData[0])), C.size_t(size-1), C.XKB_KEYMAP_FORMAT_TEXT_V1, C.XKB_KEYMAP_COMPILE_NO_FLAGS)
	if xkb.keyMap == nil {
		xkb.Destroy()
		return nil, errors.New("newXKB: xkb_keymap_new_from_buffer failed")
	}
	xkb.state = C.xkb_state_new(xkb.keyMap)
	if xkb.state == nil {
		xkb.Destroy()
		return nil, errors.New("newXKB: xkb_state_new failed")
	}
	return xkb, nil
}

func (x *xkb) dispatchKey(w *Window, keyCode C.uint32_t) {
	keyCode = mapXKBKeyCode(keyCode)
	if len(x.utf8Buf) == 0 {
		x.utf8Buf = make([]byte, 1)
	}
	sym := C.xkb_state_key_get_one_sym(x.state, C.xkb_keycode_t(keyCode))
	if n, ok := convertKeysym(sym); ok {
		cmd := key.Event{Name: n}
		if C.xkb_state_mod_name_is_active(x.state, (*C.char)(unsafe.Pointer(&_XKB_MOD_NAME_CTRL[0])), C.XKB_STATE_MODS_EFFECTIVE) == 1 {
			cmd.Modifiers |= key.ModCommand
		}
		if C.xkb_state_mod_name_is_active(x.state, (*C.char)(unsafe.Pointer(&_XKB_MOD_NAME_SHIFT[0])), C.XKB_STATE_MODS_EFFECTIVE) == 1 {
			cmd.Modifiers |= key.ModShift
		}
		w.event(cmd)
	}
	C.xkb_compose_state_feed(x.compState, sym)
	var size C.int
	switch C.xkb_compose_state_get_status(x.compState) {
	case C.XKB_COMPOSE_CANCELLED, C.XKB_COMPOSE_COMPOSING:
		return
	case C.XKB_COMPOSE_COMPOSED:
		size = C.xkb_compose_state_get_utf8(x.compState, (*C.char)(unsafe.Pointer(&x.utf8Buf[0])), C.size_t(len(x.utf8Buf)))
		if int(size) >= len(x.utf8Buf) {
			x.utf8Buf = make([]byte, size+1)
			size = C.xkb_compose_state_get_utf8(x.compState, (*C.char)(unsafe.Pointer(&x.utf8Buf[0])), C.size_t(len(x.utf8Buf)))
		}
		C.xkb_compose_state_reset(x.compState)
	case C.XKB_COMPOSE_NOTHING:
		size = C.xkb_state_key_get_utf8(x.state, C.xkb_keycode_t(keyCode), (*C.char)(unsafe.Pointer(&x.utf8Buf[0])), C.size_t(len(x.utf8Buf)))
		if int(size) >= len(x.utf8Buf) {
			x.utf8Buf = make([]byte, size+1)
			size = C.xkb_state_key_get_utf8(x.state, C.xkb_keycode_t(keyCode), (*C.char)(unsafe.Pointer(&x.utf8Buf[0])), C.size_t(len(x.utf8Buf)))
		}
	}
	// Report only printable runes.
	str := x.utf8Buf[:size]
	var n int
	for n < len(str) {
		r, s := utf8.DecodeRune(str)
		if unicode.IsPrint(r) {
			n += s
		} else {
			copy(str[n:], str[n+s:])
			str = str[:len(str)-s]
		}
	}
	if len(str) > 0 {
		w.event(key.EditEvent{Text: string(str)})
	}
}

func (x *xkb) isRepeatKey(keyCode C.uint32_t) bool {
	keyCode = mapXKBKeyCode(keyCode)
	return C.xkb_keymap_key_repeats(conn.xkb.keyMap, C.xkb_keycode_t(keyCode)) == 1
}

func (x *xkb) updateMask(depressed, latched, locked, group C.uint32_t) {
	xkbGrp := C.xkb_layout_index_t(group)
	C.xkb_state_update_mask(conn.xkb.state, C.xkb_mod_mask_t(depressed), C.xkb_mod_mask_t(latched), C.xkb_mod_mask_t(locked), xkbGrp, xkbGrp, xkbGrp)
}

func mapXKBKeyCode(keyCode C.uint32_t) C.uint32_t {
	// According to the xkb_v1 spec: "to determine the xkb keycode, clients must add 8 to the key event keycode."
	return keyCode + 8
}

func convertKeysym(s C.xkb_keysym_t) (rune, bool) {
	if '0' <= s && s <= '9' || 'A' <= s && s <= 'Z' {
		return rune(s), true
	}
	if 'a' <= s && s <= 'z' {
		return rune(s - 0x20), true
	}
	var n rune
	switch s {
	case C.XKB_KEY_Escape:
		n = key.NameEscape
	case C.XKB_KEY_Left:
		n = key.NameLeftArrow
	case C.XKB_KEY_Right:
		n = key.NameRightArrow
	case C.XKB_KEY_Return:
		n = key.NameReturn
	case C.XKB_KEY_KP_Enter:
		n = key.NameEnter
	case C.XKB_KEY_Up:
		n = key.NameUpArrow
	case C.XKB_KEY_Down:
		n = key.NameDownArrow
	case C.XKB_KEY_Home:
		n = key.NameHome
	case C.XKB_KEY_End:
		n = key.NameEnd
	case C.XKB_KEY_BackSpace:
		n = key.NameDeleteBackward
	case C.XKB_KEY_Delete:
		n = key.NameDeleteForward
	case C.XKB_KEY_Page_Up:
		n = key.NamePageUp
	case C.XKB_KEY_Page_Down:
		n = key.NamePageDown
	default:
		return 0, false
	}
	return n, true
}
