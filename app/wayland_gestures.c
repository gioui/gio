//go:build ((linux && !android) || freebsd) && !nowayland

#include "wayland_pointer_gestures.h"

extern void gio_onPinchBegin(void *data, struct zwp_pointer_gesture_pinch_v1 *obj, uint32_t serial, uint32_t time, struct wl_surface *surface, uint32_t fingers);
extern void gio_onPinchUpdate(void *data, struct zwp_pointer_gesture_pinch_v1 *obj, uint32_t time, wl_fixed_t dx, wl_fixed_t dy, wl_fixed_t scale, wl_fixed_t rotation);
extern void gio_onPinchEnd(void *data, struct zwp_pointer_gesture_pinch_v1 *obj, uint32_t serial, uint32_t time, int32_t cancelled);

extern void gio_onSwipeBegin(void *data, struct zwp_pointer_gesture_swipe_v1 *obj, uint32_t serial, uint32_t time, struct wl_surface *surface, uint32_t fingers);
extern void gio_onSwipeUpdate(void *data, struct zwp_pointer_gesture_swipe_v1 *obj, uint32_t time, wl_fixed_t dx, wl_fixed_t dy);
extern void gio_onSwipeEnd(void *data, struct zwp_pointer_gesture_swipe_v1 *obj, uint32_t serial, uint32_t time, int32_t cancelled);

extern void gio_onHoldBegin(void *data, struct zwp_pointer_gesture_hold_v1 *obj, uint32_t serial, uint32_t time, struct wl_surface *surface, uint32_t fingers);
extern void gio_onHoldEnd(void *data, struct zwp_pointer_gesture_hold_v1 *obj, uint32_t serial, uint32_t time, int32_t cancelled);

const struct zwp_pointer_gesture_pinch_v1_listener gio_pinch_listener = {
	.begin = gio_onPinchBegin,
	.update = gio_onPinchUpdate,
	.end = gio_onPinchEnd,
};

const struct zwp_pointer_gesture_swipe_v1_listener gio_swipe_listener = {
	.begin = gio_onSwipeBegin,
	.update = gio_onSwipeUpdate,
	.end = gio_onSwipeEnd,
};

const struct zwp_pointer_gesture_hold_v1_listener gio_hold_listener = {
	.begin = gio_onHoldBegin,
	.end = gio_onHoldEnd,
};
