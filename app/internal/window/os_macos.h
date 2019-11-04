// SPDX-License-Identifier: Unlicense OR MIT

#ifndef _OS_MACOS_H
#define _OS_MACOS_H

#define GIO_MOUSE_MOVE 1
#define GIO_MOUSE_UP 2
#define GIO_MOUSE_DOWN 3

__attribute__ ((visibility ("hidden"))) void gio_main(CFTypeRef viewRef, const char *title, CGFloat width, CGFloat height);
__attribute__ ((visibility ("hidden"))) CGFloat gio_viewWidth(CFTypeRef viewRef);
__attribute__ ((visibility ("hidden"))) CGFloat gio_viewHeight(CFTypeRef viewRef);
__attribute__ ((visibility ("hidden"))) void gio_setAnimating(CFTypeRef viewRef, BOOL anim);
__attribute__ ((visibility ("hidden"))) void gio_updateDisplayLink(CFTypeRef viewRef, CGDirectDisplayID dispID);
__attribute__ ((visibility ("hidden"))) CGFloat gio_getViewBackingScale(CFTypeRef viewRef);

#endif
