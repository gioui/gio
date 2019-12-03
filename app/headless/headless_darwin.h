// SPDX-License-Identifier: Unlicense OR MIT

__attribute__ ((visibility ("hidden"))) CFTypeRef gio_headless_newContext(void);
__attribute__ ((visibility ("hidden"))) void gio_headless_releaseContext(CFTypeRef ctxRef);
__attribute__ ((visibility ("hidden"))) void gio_headless_clearCurrentContext(CFTypeRef ctxRef);
__attribute__ ((visibility ("hidden"))) void gio_headless_makeCurrentContext(CFTypeRef ctxRef);
__attribute__ ((visibility ("hidden"))) void gio_headless_prepareContext(CFTypeRef ctxRef);
