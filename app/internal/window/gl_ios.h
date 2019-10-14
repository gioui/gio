// SPDX-License-Identifier: Unlicense OR MIT

__attribute__ ((visibility ("hidden"))) int gio_renderbufferStorage(CFTypeRef ctx, CFTypeRef layer, GLenum buffer);
__attribute__ ((visibility ("hidden"))) int gio_presentRenderbuffer(CFTypeRef ctx, GLenum buffer);
__attribute__ ((visibility ("hidden"))) int gio_makeCurrent(CFTypeRef ctx);
__attribute__ ((visibility ("hidden"))) CFTypeRef gio_createContext(void);
__attribute__ ((visibility ("hidden"))) CFTypeRef gio_createGLLayer(void);
