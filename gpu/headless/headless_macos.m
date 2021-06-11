// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,!ios

@import AppKit;
@import OpenGL;
@import OpenGL.GL;
@import OpenGL.GL3;

#include <CoreFoundation/CoreFoundation.h>
#include "_cgo_export.h"

void gio_headless_releaseContext(CFTypeRef ctxRef) {
	CFBridgingRelease(ctxRef);
}

CFTypeRef gio_headless_newContext(void) {
	NSOpenGLPixelFormatAttribute attr[] = {
		NSOpenGLPFAOpenGLProfile, NSOpenGLProfileVersion3_2Core,
		NSOpenGLPFAColorSize,     24,
		NSOpenGLPFAAccelerated,
		// Opt-in to automatic GPU switching. CGL-only property.
		kCGLPFASupportsAutomaticGraphicsSwitching,
		NSOpenGLPFAAllowOfflineRenderers,
		0
	};
	NSOpenGLPixelFormat *pixFormat = [[NSOpenGLPixelFormat alloc] initWithAttributes:attr];
	if (pixFormat == nil) {
		return NULL;
	}
	NSOpenGLContext *ctx = [[NSOpenGLContext alloc] initWithFormat:pixFormat shareContext:nil];
	return CFBridgingRetain(ctx);
}

void gio_headless_clearCurrentContext(CFTypeRef ctxRef) {
	NSOpenGLContext *ctx = (__bridge NSOpenGLContext *)ctxRef;
	CGLUnlockContext([ctx CGLContextObj]);
	[NSOpenGLContext clearCurrentContext];
}

void gio_headless_makeCurrentContext(CFTypeRef ctxRef) {
	NSOpenGLContext *ctx = (__bridge NSOpenGLContext *)ctxRef;
	[ctx makeCurrentContext];
	CGLLockContext([ctx CGLContextObj]);
}
