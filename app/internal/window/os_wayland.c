// SPDX-License-Identifier: Unlicense OR MIT

// +build linux,!android,!nowayland freebsd

#include <wayland-client.h>
#include "wayland_xdg_shell.h"
#include "wayland_text_input.h"
#include "os_wayland.h"
#include "_cgo_export.h"

static const struct wl_registry_listener registry_listener = {
	// Cast away const parameter.
	.global = (void (*)(void *, struct wl_registry *, uint32_t,  const char *, uint32_t))gio_onRegistryGlobal,
	.global_remove = gio_onRegistryGlobalRemove
};

void gio_wl_registry_add_listener(struct wl_registry *reg) {
	wl_registry_add_listener(reg, &registry_listener, NULL);
}

static struct wl_surface_listener surface_listener = {.enter = gio_onSurfaceEnter, .leave = gio_onSurfaceLeave};

void gio_wl_surface_add_listener(struct wl_surface *surface) {
	wl_surface_add_listener(surface, &surface_listener, NULL);
}

static const struct xdg_surface_listener xdg_surface_listener = {
	.configure = gio_onXdgSurfaceConfigure,
};

void gio_xdg_surface_add_listener(struct xdg_surface *surface) {
	xdg_surface_add_listener(surface, &xdg_surface_listener, NULL);
}

static const struct xdg_toplevel_listener xdg_toplevel_listener = {
	.configure = gio_onToplevelConfigure,
	.close = gio_onToplevelClose,
};

void gio_xdg_toplevel_add_listener(struct xdg_toplevel *toplevel) {
	xdg_toplevel_add_listener(toplevel, &xdg_toplevel_listener, NULL);
}

static void xdg_wm_base_handle_ping(void *data, struct xdg_wm_base *wm, uint32_t serial) {
	xdg_wm_base_pong(wm, serial);
}
static const struct xdg_wm_base_listener xdg_wm_base_listener = {
	.ping = xdg_wm_base_handle_ping,
};

void gio_xdg_wm_base_add_listener(struct xdg_wm_base *wm) {
	xdg_wm_base_add_listener(wm, &xdg_wm_base_listener, NULL);
}

static const struct wl_callback_listener wl_callback_listener = {
	.done = gio_onFrameDone,
};

void gio_wl_callback_add_listener(struct wl_callback *callback, void *data) {
	wl_callback_add_listener(callback, &wl_callback_listener, data);
}

static const struct wl_output_listener wl_output_listener = {
	// Cast away const parameter.
	.geometry = (void (*)(void *, struct wl_output *, int32_t,  int32_t,  int32_t,  int32_t,  int32_t,  const char *, const char *, int32_t))gio_onOutputGeometry,
	.mode = gio_onOutputMode,
	.done = gio_onOutputDone,
	.scale = gio_onOutputScale,
};

void gio_wl_output_add_listener(struct wl_output *output) {
	wl_output_add_listener(output, &wl_output_listener, NULL);
}

static const struct wl_seat_listener wl_seat_listener = {
	.capabilities = gio_onSeatCapabilities,
	// Cast away const parameter.
	.name = (void (*)(void *, struct wl_seat *, const char *))gio_onSeatName,
};

void gio_wl_seat_add_listener(struct wl_seat *seat) {
	wl_seat_add_listener(seat, &wl_seat_listener, NULL);
}

static const struct wl_pointer_listener wl_pointer_listener = {
	.enter = gio_onPointerEnter,
	.leave = gio_onPointerLeave,
	.motion = gio_onPointerMotion,
	.button = gio_onPointerButton,
	.axis = gio_onPointerAxis,
	.frame = gio_onPointerFrame,
	.axis_source = gio_onPointerAxisSource,
	.axis_stop = gio_onPointerAxisStop,
	.axis_discrete = gio_onPointerAxisDiscrete,
};

void gio_wl_pointer_add_listener(struct wl_pointer *pointer) {
	wl_pointer_add_listener(pointer, &wl_pointer_listener, NULL);
}

static const struct wl_touch_listener wl_touch_listener = {
	.down = gio_onTouchDown,
	.up = gio_onTouchUp,
	.motion = gio_onTouchMotion,
	.frame = gio_onTouchFrame,
	.cancel = gio_onTouchCancel,
};

void gio_wl_touch_add_listener(struct wl_touch *touch) {
	wl_touch_add_listener(touch, &wl_touch_listener, NULL);
}

static const struct wl_keyboard_listener wl_keyboard_listener = {
	.keymap = gio_onKeyboardKeymap,
	.enter = gio_onKeyboardEnter,
	.leave = gio_onKeyboardLeave,
	.key = gio_onKeyboardKey,
	.modifiers = gio_onKeyboardModifiers,
	.repeat_info = gio_onKeyboardRepeatInfo
};

void gio_wl_keyboard_add_listener(struct wl_keyboard *keyboard) {
	wl_keyboard_add_listener(keyboard, &wl_keyboard_listener, NULL);
}

static const struct zwp_text_input_v3_listener zwp_text_input_v3_listener = {
	.enter = gio_onTextInputEnter,
	.leave = gio_onTextInputLeave,
	// Cast away const parameter.
	.preedit_string = (void (*)(void *, struct zwp_text_input_v3 *, const char *, int32_t,  int32_t))gio_onTextInputPreeditString,
	.commit_string = (void (*)(void *, struct zwp_text_input_v3 *, const char *))gio_onTextInputCommitString,
	.delete_surrounding_text = gio_onTextInputDeleteSurroundingText,
	.done = gio_onTextInputDone
};

void gio_zwp_text_input_v3_add_listener(struct zwp_text_input_v3 *im) {
	zwp_text_input_v3_add_listener(im, &zwp_text_input_v3_listener, NULL);
}
