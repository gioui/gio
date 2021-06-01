// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,!ios

@import AppKit;

#include <CoreFoundation/CoreFoundation.h>
#include <OpenGL/OpenGL.h>
#include "_cgo_export.h"

@interface GioGLContext : NSOpenGLContext
@end

@implementation GioGLContext
- (void) notifyUpdate:(NSNotification*)notification {
	CGLLockContext([self CGLContextObj]);
	[self update];
	CGLUnlockContext([self CGLContextObj]);
}
- (void)dealloc {
	[[NSNotificationCenter defaultCenter] removeObserver:self];
}
@end

CFTypeRef gio_createGLContext(void) {
	@autoreleasepool {
		NSOpenGLPixelFormatAttribute attr[] = {
			NSOpenGLPFAOpenGLProfile, NSOpenGLProfileVersion3_2Core,
			NSOpenGLPFAColorSize,     24,
			NSOpenGLPFADepthSize,     16,
			NSOpenGLPFAAccelerated,
			// Opt-in to automatic GPU switching. CGL-only property.
			kCGLPFASupportsAutomaticGraphicsSwitching,
			NSOpenGLPFAAllowOfflineRenderers,
			0
		};
		NSOpenGLPixelFormat *pixFormat = [[NSOpenGLPixelFormat alloc] initWithAttributes:attr];

		GioGLContext *ctx = [[GioGLContext alloc] initWithFormat:pixFormat shareContext: nil];
		return CFBridgingRetain(ctx);
	}
}

void gio_setContextView(CFTypeRef ctxRef, CFTypeRef viewRef) {
	GioGLContext *ctx = (__bridge GioGLContext *)ctxRef;
	NSView *view = (__bridge NSView *)viewRef;
	[view setWantsBestResolutionOpenGLSurface:YES];
	[ctx setView:view];
	[[NSNotificationCenter defaultCenter] addObserver:ctx
											 selector:@selector(notifyUpdate:)
												 name:NSViewGlobalFrameDidChangeNotification
											   object:view];
}

void gio_clearCurrentContext(void) {
	@autoreleasepool {
		[NSOpenGLContext clearCurrentContext];
	}
}

void gio_makeCurrentContext(CFTypeRef ctxRef) {
	@autoreleasepool {
		NSOpenGLContext *ctx = (__bridge NSOpenGLContext *)ctxRef;
		[ctx makeCurrentContext];
	}
}

void gio_lockContext(CFTypeRef ctxRef) {
	@autoreleasepool {
		NSOpenGLContext *ctx = (__bridge NSOpenGLContext *)ctxRef;
		CGLLockContext([ctx CGLContextObj]);
	}
}

void gio_unlockContext(CFTypeRef ctxRef) {
	@autoreleasepool {
		NSOpenGLContext *ctx = (__bridge NSOpenGLContext *)ctxRef;
		CGLUnlockContext([ctx CGLContextObj]);
	}
}
