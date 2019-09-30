// SPDX-License-Identifier: Unlicense OR MIT

__attribute__ ((visibility ("hidden"))) void gio_wl_registry_add_listener(struct wl_registry *reg);
__attribute__ ((visibility ("hidden"))) void gio_wl_surface_add_listener(struct wl_surface *surface);
__attribute__ ((visibility ("hidden"))) void gio_xdg_surface_add_listener(struct xdg_surface *surface);
__attribute__ ((visibility ("hidden"))) void gio_xdg_toplevel_add_listener(struct xdg_toplevel *toplevel);
__attribute__ ((visibility ("hidden"))) void gio_xdg_wm_base_add_listener(struct xdg_wm_base *wm);
__attribute__ ((visibility ("hidden"))) void gio_wl_callback_add_listener(struct wl_callback *callback, void *data);
__attribute__ ((visibility ("hidden"))) void gio_wl_output_add_listener(struct wl_output *output);
__attribute__ ((visibility ("hidden"))) void gio_wl_seat_add_listener(struct wl_seat *seat);
__attribute__ ((visibility ("hidden"))) void gio_wl_pointer_add_listener(struct wl_pointer *pointer);
__attribute__ ((visibility ("hidden"))) void gio_wl_touch_add_listener(struct wl_touch *touch);
__attribute__ ((visibility ("hidden"))) void gio_wl_keyboard_add_listener(struct wl_keyboard *keyboard);
__attribute__ ((visibility ("hidden"))) void gio_zwp_text_input_v3_add_listener(struct zwp_text_input_v3 *im);
