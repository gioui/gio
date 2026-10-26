// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony

// Definitions live in their own translation unit: a cgo file that uses
// //export must keep its preamble free of definitions, or the symbols are
// emitted twice.
#include <string.h>

#include <native_buffer/buffer_common.h>
#include <native_window/external_window.h>

#include "ohos_glue.h"

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

static gio_ohos_host gio_host;
static int gio_host_ready;

void gio_ohos_store_host(const gio_ohos_host *host) {
    if (host == NULL) {
        memset(&gio_host, 0, sizeof(gio_host));
        gio_host_ready = 0;
        return;
    }
    gio_host = *host;
    gio_host_ready = 1;
}

int gio_ohos_host_ready(void) {
    return gio_host_ready;
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
