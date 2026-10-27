// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony

// Definitions live in their own translation unit: a cgo file that uses
// //export must keep its preamble free of definitions, or the symbols are
// emitted twice.
#include <string.h>

#ifndef GIO_OHOS_HOST_TEST
#include <native_buffer/buffer_common.h>
#include <native_window/external_window.h>
#endif

#include "ohos_glue.h"

#ifndef GIO_OHOS_HOST_TEST
void gio_ohos_set_buffer_geometry(void *window, int width, int height) {
    if (window == NULL) {
        return;
    }
    OH_NativeWindow_NativeWindowHandleOpt((OHNativeWindow *)window, SET_BUFFER_GEOMETRY,
                                         (int32_t)width, (int32_t)height);
}

void gio_ohos_set_buffer_format(void *window, int format) {
    if (window == NULL) {
        return;
    }
    OH_NativeWindow_NativeWindowHandleOpt((OHNativeWindow *)window, SET_FORMAT, (int32_t)format);
}
#endif

static gio_ohos_host_v4 gio_host_v4;
#define gio_host_v3 (gio_host_v4.v3)
static int gio_host_ready;
static int gio_host_abi;

/* Existing callbacks keep their short spelling. */
#define gio_host (gio_host_v3.v2)

void gio_ohos_store_host(const gio_ohos_host *host) {
    memset(&gio_host_v3, 0, sizeof(gio_host_v3));
    if (host == NULL) {
        gio_host_ready = 0;
        gio_host_abi = 0;
        return;
    }
    memcpy(&gio_host_v3.v2, host, sizeof(*host));
    gio_host_ready = 1;
    gio_host_abi = GIO_OHOS_ABI2_VERSION;
}

void gio_ohos_store_host_v4(const gio_ohos_host_v4 *host, size_t size) {
    memset(&gio_host_v4, 0, sizeof(gio_host_v4));
    if (host == NULL || size == 0) {
        gio_host_ready = 0;
        gio_host_abi = 0;
        return;
    }
    size_t n = size < sizeof(gio_host_v4) ? size : sizeof(gio_host_v4);
    memcpy(&gio_host_v4, host, n);
    gio_host_ready = 1;
    gio_host_abi = n > offsetof(gio_ohos_host_v4, select_window)
                       ? GIO_OHOS_ABI_VERSION
                       : GIO_OHOS_ABI3_VERSION;
}

void gio_ohos_store_host_v3(const gio_ohos_host_v3 *host, size_t size) {
    memset(&gio_host_v3, 0, sizeof(gio_host_v3));
    if (host == NULL || size == 0) {
        gio_host_ready = 0;
        gio_host_abi = 0;
        return;
    }
    size_t n = size < sizeof(gio_host_v3) ? size : sizeof(gio_host_v3);
    memcpy(&gio_host_v3, host, n);
    gio_host_ready = 1;
    gio_host_abi = n > offsetof(gio_ohos_host_v3, set_window_mode)
                       ? GIO_OHOS_ABI3_VERSION
                       : GIO_OHOS_ABI2_VERSION;
}

int gio_ohos_host_ready(void) {
    return gio_host_ready;
}

int gio_ohos_host_abi_version(void) {
    return gio_host_abi;
}

void gio_call_select_window(uint64_t windowID) {
    if (gio_host_ready && gio_host_abi >= GIO_OHOS_ABI_VERSION && gio_host_v4.select_window != NULL) {
        gio_host_v4.select_window(gio_host_v4.v3.v2.user, windowID);
    }
}

void gio_call_create_window(uint64_t windowID) {
    if (gio_host_ready && gio_host_abi >= GIO_OHOS_ABI_VERSION && gio_host_v4.create_window != NULL) {
        gio_host_v4.create_window(gio_host_v4.v3.v2.user, windowID);
    }
}

void gio_call_destroy_window(uint64_t windowID) {
    if (gio_host_ready && gio_host_abi >= GIO_OHOS_ABI_VERSION && gio_host_v4.destroy_window != NULL) {
        gio_host_v4.destroy_window(gio_host_v4.v3.v2.user, windowID);
    }
}

#define GIO_HOST_CALL(fn, ...) do { \
    if (gio_host_ready && gio_host.fn != NULL) { \
        gio_host.fn(gio_host.user, __VA_ARGS__); \
    } \
} while (0)

#define GIO_HOST_CALL0(fn) do { \
    if (gio_host_ready && gio_host.fn != NULL) { \
        gio_host.fn(gio_host.user); \
    } \
} while (0)

void gio_call_show_keyboard(int show) {
    GIO_HOST_CALL(show_keyboard, show);
}

void gio_call_set_clipboard(const char *mime, const char *text) {
    GIO_HOST_CALL(set_clipboard, mime, text);
}

int gio_call_set_clipboard_data(const char *mime, const void *data, int length) {
    if (gio_host_ready && gio_host_abi >= GIO_OHOS_ABI3_VERSION && gio_host_v3.set_clipboard_data != NULL) {
        gio_host_v3.set_clipboard_data(gio_host_v3.v2.user, mime, data, length);
        return 1;
    }
    return 0;
}

void gio_call_get_clipboard(const char *mime) {
    GIO_HOST_CALL(get_clipboard, mime);
}

void gio_call_set_cursor(int cursor) {
    GIO_HOST_CALL(set_cursor, cursor);
}

void gio_call_invalidate(void) {
    GIO_HOST_CALL0(invalidate);
}

void gio_call_pick_file(int mode) {
    GIO_HOST_CALL(pick_file, mode);
}

void gio_call_log(const char *msg) {
    GIO_HOST_CALL(log, msg);
}

void gio_call_ime_selection(const char *text, int length, int start, int end) {
    GIO_HOST_CALL(ime_selection, text, length, start, end);
}

void gio_call_ime_cursor(float x, float y, float width, float height) {
    GIO_HOST_CALL(ime_cursor, x, y, width, height);
}

void gio_call_ime_config(int hint) {
    GIO_HOST_CALL(ime_config, hint);
}

void gio_call_ime_keyboard_status(int status) {
    GIO_HOST_CALL(ime_keyboard_status, status);
}

#define GIO_HOST3_CALL(fn, ...) do { \
    if (gio_host_ready && gio_host_abi >= GIO_OHOS_ABI3_VERSION && gio_host_v3.fn != NULL) { \
        gio_host_v3.fn(gio_host_v3.v2.user, __VA_ARGS__); \
    } \
} while (0)

#define GIO_HOST3_CALL0(fn) do { \
    if (gio_host_ready && gio_host_abi >= GIO_OHOS_ABI3_VERSION && gio_host_v3.fn != NULL) { \
        gio_host_v3.fn(gio_host_v3.v2.user); \
    } \
} while (0)

void gio_call_set_window_mode(int mode) {
    GIO_HOST3_CALL(set_window_mode, mode);
}

void gio_call_set_title(const char *title) {
    GIO_HOST3_CALL(set_title, title);
}

void gio_call_set_orientation(int orientation) {
    GIO_HOST3_CALL(set_orientation, orientation);
}

void gio_call_set_status_color(uint32_t argb) {
    GIO_HOST3_CALL(set_status_color, argb);
}

void gio_call_set_navigation_color(uint32_t argb) {
    GIO_HOST3_CALL(set_navigation_color, argb);
}

void gio_call_set_top_most(int topmost) {
    GIO_HOST3_CALL(set_top_most, topmost);
}

void gio_call_set_window_limits(int minw, int minh, int maxw, int maxh) {
    GIO_HOST3_CALL(set_window_limits, minw, minh, maxw, maxh);
}

void gio_call_pick_file_ex(int mode, const char *mimes, int multiple) {
    GIO_HOST3_CALL(pick_file_ex, mode, mimes, multiple);
}

int gio_call_request_permission(const char *name, int requestID) {
    if (gio_host_ready && gio_host_abi >= GIO_OHOS_ABI3_VERSION && gio_host_v3.request_permission != NULL) {
        gio_host_v3.request_permission(gio_host_v3.v2.user, name, requestID);
        return 1;
    }
    return 0;
}

int gio_call_request_close(void) {
    if (gio_host_ready && gio_host_abi >= GIO_OHOS_ABI3_VERSION && gio_host_v3.request_close != NULL) {
        gio_host_v3.request_close(gio_host_v3.v2.user);
        return 1;
    }
    return 0;
}

void gio_call_raise_window(void) {
    GIO_HOST3_CALL0(raise_window);
}

void gio_call_center_window(void) {
    GIO_HOST3_CALL0(center_window);
}

void gio_call_start_window_move(void) {
    GIO_HOST3_CALL0(start_window_move);
}

void gio_call_set_animating(int animating) {
    GIO_HOST3_CALL(set_animating, animating);
}

void gio_call_semantics_update(const char *json, int length) {
    GIO_HOST3_CALL(semantics_update, json, length);
}

void gio_call_semantics_announce(const char *text) {
    GIO_HOST3_CALL(semantics_announce, text);
}
