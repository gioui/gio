// SPDX-License-Identifier: Unlicense OR MIT

// +build linux,!android,!nowayland freebsd

#include <wayland-client.h>
#include "wayland_xdg_shell.h"
#include "wayland_text_input.h"
#include "_cgo_export.h"

void gio_wl_registry_add_listener(struct wl_registry *reg, void *data) {
	static const struct wl_registry_listener listener = {
		// Cast away const parameter.
		.global = (void (*)(void *, struct wl_registry *, uint32_t,  const char *, uint32_t))gio_onRegistryGlobal,
		.global_remove = gio_onRegistryGlobalRemove
	};

	wl_registry_add_listener(reg, &listener, data);
}

void gio_wl_surface_add_listener(struct wl_surface *surface, void *data) {
	static struct wl_surface_listener listener = {
		.enter = gio_onSurfaceEnter,
		.leave = gio_onSurfaceLeave,
	};

	wl_surface_add_listener(surface, &listener, data);
}

void gio_xdg_surface_add_listener(struct xdg_surface *surface, void *data) {
	static const struct xdg_surface_listener listener = {
		.configure = gio_onXdgSurfaceConfigure,
	};

	xdg_surface_add_listener(surface, &listener, data);
}

void gio_xdg_toplevel_add_listener(struct xdg_toplevel *toplevel, void *data) {
	static const struct xdg_toplevel_listener listener = {
		.configure = gio_onToplevelConfigure,
		.close = gio_onToplevelClose,
	};

	xdg_toplevel_add_listener(toplevel, &listener, data);
}

static void xdg_wm_base_handle_ping(void *data, struct xdg_wm_base *wm, uint32_t serial) {
	xdg_wm_base_pong(wm, serial);
}

void gio_xdg_wm_base_add_listener(struct xdg_wm_base *wm, void *data) {
	static const struct xdg_wm_base_listener listener = {
		.ping = xdg_wm_base_handle_ping,
	};

	xdg_wm_base_add_listener(wm, &listener, data);
}

void gio_wl_callback_add_listener(struct wl_callback *callback, void *data) {
	static const struct wl_callback_listener listener = {
		.done = gio_onFrameDone,
	};

	wl_callback_add_listener(callback, &listener, data);
}

void gio_wl_output_add_listener(struct wl_output *output, void *data) {
	static const struct wl_output_listener listener = {
		// Cast away const parameter.
		.geometry = (void (*)(void *, struct wl_output *, int32_t,  int32_t,  int32_t,  int32_t,  int32_t,  const char *, const char *, int32_t))gio_onOutputGeometry,
		.mode = gio_onOutputMode,
		.done = gio_onOutputDone,
		.scale = gio_onOutputScale,
	};

	wl_output_add_listener(output, &listener, data);
}

void gio_wl_seat_add_listener(struct wl_seat *seat, void *data) {
	static const struct wl_seat_listener listener = {
		.capabilities = gio_onSeatCapabilities,
		// Cast away const parameter.
		.name = (void (*)(void *, struct wl_seat *, const char *))gio_onSeatName,
	};

	wl_seat_add_listener(seat, &listener, data);
}

void gio_wl_pointer_add_listener(struct wl_pointer *pointer, void *data) {
	static const struct wl_pointer_listener listener = {
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

	wl_pointer_add_listener(pointer, &listener, data);
}

void gio_wl_touch_add_listener(struct wl_touch *touch, void *data) {
	static const struct wl_touch_listener listener = {
		.down = gio_onTouchDown,
		.up = gio_onTouchUp,
		.motion = gio_onTouchMotion,
		.frame = gio_onTouchFrame,
		.cancel = gio_onTouchCancel,
	};

	wl_touch_add_listener(touch, &listener, data);
}

void gio_wl_keyboard_add_listener(struct wl_keyboard *keyboard, void *data) {
	static const struct wl_keyboard_listener listener = {
		.keymap = gio_onKeyboardKeymap,
		.enter = gio_onKeyboardEnter,
		.leave = gio_onKeyboardLeave,
		.key = gio_onKeyboardKey,
		.modifiers = gio_onKeyboardModifiers,
		.repeat_info = gio_onKeyboardRepeatInfo
	};

	wl_keyboard_add_listener(keyboard, &listener, data);
}

void gio_zwp_text_input_v3_add_listener(struct zwp_text_input_v3 *im, void *data) {
	static const struct zwp_text_input_v3_listener listener = {
		.enter = gio_onTextInputEnter,
		.leave = gio_onTextInputLeave,
		// Cast away const parameter.
		.preedit_string = (void (*)(void *, struct zwp_text_input_v3 *, const char *, int32_t,  int32_t))gio_onTextInputPreeditString,
		.commit_string = (void (*)(void *, struct zwp_text_input_v3 *, const char *))gio_onTextInputCommitString,
		.delete_surrounding_text = gio_onTextInputDeleteSurroundingText,
		.done = gio_onTextInputDone
	};

	zwp_text_input_v3_add_listener(im, &listener, data);
}
