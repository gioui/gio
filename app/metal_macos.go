// SPDX-License-Identifier: Unlicense OR MIT

//go:build darwin && !ios && !nometal
// +build darwin,!ios,!nometal

package app

/*
#cgo CFLAGS: -Werror -xobjective-c -fobjc-arc

#import <AppKit/AppKit.h>
#import <QuartzCore/CAMetalLayer.h>
#include <CoreFoundation/CoreFoundation.h>

CALayer *gio_layerFactory(void) {
	@autoreleasepool {
		CAMetalLayer *l = [CAMetalLayer layer];
		l.autoresizingMask = kCALayerHeightSizable|kCALayerWidthSizable;
		l.needsDisplayOnBoundsChange = YES;
		l.presentsWithTransaction = YES;
		return l;
	}
}

static CFTypeRef getMetalLayer(CFTypeRef viewRef) {
	@autoreleasepool {
		NSView *view = (__bridge NSView *)viewRef;
		return CFBridgingRetain(view.layer);
	}
}

static void resizeDrawable(CFTypeRef viewRef, CFTypeRef layerRef) {
	@autoreleasepool {
		NSView *view = (__bridge NSView *)viewRef;
		CAMetalLayer *layer = (__bridge CAMetalLayer *)layerRef;
		CGSize size = layer.bounds.size;
		size.width *= layer.contentsScale;
		size.height *= layer.contentsScale;
		layer.drawableSize = size;
	}
}
*/
import "C"

func getMetalLayer(view C.CFTypeRef) C.CFTypeRef {
	return C.getMetalLayer(view)
}

func resizeDrawable(view, layer C.CFTypeRef) {
	C.resizeDrawable(view, layer)
}
