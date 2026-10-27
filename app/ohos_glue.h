// SPDX-License-Identifier: Unlicense OR MIT

// gio_ohos_glue.h is the stable C ABI between the Gio OHOS backend (this
// package, loaded as libgpofd.so) and the HarmonyOS napi shim
// (libgpofdohos.so).  It is deliberately free of napi/ArkTS headers so the
// backend can be built and tested without the HarmonyOS SDK.
//
// ABI 1 entries are frozen (14 exports, see docs/ohos-remediation-plan.md §5).  ABI 2 adds a small
// set of *compatible* extensions: callers must not assume them to be present
// (the shim may be older), and receivers must tolerate NULL host pointers.

#ifndef GIO_OHOS_GLUE_H
#define GIO_OHOS_GLUE_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define GIO_OHOS_ABI_VERSION 2
#define GIO_OHOS_ABI1_ENTRIES 14

/* Host callback table, implemented by the napi shim. */
typedef struct gio_ohos_host {
    void *user;

    /* ---- ABI 1 (frozen) ---- */
    void (*show_keyboard)(void *user, int show);
    void (*set_clipboard)(void *user, const char *mime, const char *text);
    void (*get_clipboard)(void *user, const char *mime);
    void (*set_cursor)(void *user, int cursor);
    void (*invalidate)(void *user);
    void (*pick_file)(void *user, int mode);
    void (*log)(void *user, const char *msg);

    /* ---- ABI 2 extensions (may be NULL) ---- */
    /* Report the surrounding text (UTF-8, `length` bytes) and selection
       offsets (runes relative to `text`). */
    void (*ime_selection)(void *user, const char *text, int length, int start, int end);
    /* Caret rectangle in surface pixels. */
    void (*ime_cursor)(void *user, float x, float y, float width, float height);
    /* Gio key.InputHint as integer (0 = any). */
    void (*ime_config)(void *user, int hint);
    /* 0 = hidden, 1 = shown. */
    void (*ime_keyboard_status)(void *user, int status);
} gio_ohos_host;

/* Copies the host table into static storage.  Must be called before
 * gio_ohos_run; later calls replace the table. */
void gio_ohos_store_host(const gio_ohos_host *host);
int gio_ohos_host_ready(void);

/* Thin wrappers around OH_NativeWindow_NativeWindowHandleOpt (variadic,
 * which cgo cannot call directly).  They live in this translation unit so
 * the Go side stays free of platform headers. */
void gio_ohos_set_buffer_geometry(void *window, int width, int height);
void gio_ohos_set_buffer_format(void *window, int format);

/* Calls into the host table.  All are safe to call from any goroutine and
 * tolerate a missing host/entry. */
void gio_call_show_keyboard(int show);
void gio_call_set_clipboard(const char *mime, const char *text);
void gio_call_get_clipboard(const char *mime);
void gio_call_set_cursor(int cursor);
void gio_call_invalidate(void);
void gio_call_pick_file(int mode);
void gio_call_log(const char *msg);
void gio_call_ime_selection(const char *text, int length, int start, int end);
void gio_call_ime_cursor(float x, float y, float width, float height);
void gio_call_ime_config(int hint);
void gio_call_ime_keyboard_status(int status);

/* ---- Exported by Go (libgpofd.so) ----
 *
 * The shim calls these from the ArkUI/JS thread.  Every entry copies its
 * arguments and returns immediately; it never blocks on the Go event loop.
 */

/* ABI 1: install_host / configure / run / surface_* / touch / axis / key /
 * text / focus / clipboard / lifecycle / log (14). */
void gio_ohos_install_host(gio_ohos_host *host);
void gio_ohos_configure(char *dataDir, char *cacheDir, float density, float fontScale);
void gio_ohos_run(void);
void gio_ohos_surface_created(void *window, int width, int height);
void gio_ohos_surface_changed(void *window, int width, int height);
void gio_ohos_surface_destroyed(void);
void gio_ohos_touch(int action, int id, float x, float y, int mods);
void gio_ohos_axis(float x, float y, float dx, float dy, int mods);
/* Pinch: scale is relative to 1.0 (opening > 1). */
void gio_ohos_pinch(float x, float y, float scale, int mods);
void gio_ohos_key(int code, int action, int mods);
void gio_ohos_text(char *utf8);
void gio_ohos_focus(int focused);
void gio_ohos_clipboard(char *mime, char *text);
void gio_ohos_lifecycle(int state); /* 0 background, 1 foreground, 2 destroy */
void gio_ohos_log(char *msg);

/* ABI 2 extensions. */
/* Mouse: action 0 press / 1 release / 2 move; button 1 left 2 right 3 middle. */
void gio_ohos_mouse(int action, int button, float x, float y, int mods);
void gio_ohos_frame(void);                    /* vsync tick */
void gio_ohos_insets(int top, int right, int bottom, int left); /* px */
void gio_ohos_preedit(char *utf8, int selStart, int selEnd);
void gio_ohos_ime_delete(int before, int after);
void gio_ohos_ime_move(int direction);        /* 1 up 2 down 3 left 4 right */
void gio_ohos_ime_set_selection(int start, int end);
void gio_ohos_ime_enter(int type);
void gio_ohos_ime_keyboard(int status);       /* 0 hidden 1 shown */
void gio_ohos_pick_result(int mode, char *uris); /* '\n' separated */
void gio_ohos_pick_error(int mode, char *msg);

#ifdef __cplusplus
}
#endif

#endif /* GIO_OHOS_GLUE_H */
