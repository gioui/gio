// SPDX-License-Identifier: Unlicense OR MIT

__attribute__ ((visibility ("hidden"))) CFTypeRef gio_createGLView();
__attribute__ ((visibility ("hidden"))) CFTypeRef gio_contextForView(CFTypeRef viewRef);
__attribute__ ((visibility ("hidden"))) void gio_makeCurrentContext(CFTypeRef ctx);
__attribute__ ((visibility ("hidden"))) void gio_flushContextBuffer(CFTypeRef ctx);
__attribute__ ((visibility ("hidden"))) void gio_clearCurrentContext();
