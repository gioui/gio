// SPDX-License-Identifier: Unlicense OR MIT

@import Dispatch;
@import Foundation;

#include "_cgo_export.h"

void gio_wakeupMainThread(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		gio_dispatchMainFuncs();
	});
}

NSUInteger gio_nsstringLength(CFTypeRef cstr) {
	NSString *str = (__bridge NSString *)cstr;
	return [str length];
}

void gio_nsstringGetCharacters(CFTypeRef cstr, unichar *chars, NSUInteger loc, NSUInteger length) {
	NSString *str = (__bridge NSString *)cstr;
	[str getCharacters:chars range:NSMakeRange(loc, length)];
}

void gio_hideCursor() {
    @autoreleasepool {
        [NSCursor hide];
    }
}

void gio_showCursor() {
    @autoreleasepool {
        [NSCursor unhide];
    }
}

void gio_setCursor(NSUInteger curID) {
    @autoreleasepool {
        switch (curID) {
        case 1:
            [NSCursor.arrowCursor set];
            break;
        case 2:
            [NSCursor.IBeamCursor set];
            break;
        case 3:
            [NSCursor.pointingHandCursor set];
            break;
        case 4:
            [NSCursor.crosshairCursor set];
            break;
        case 5:
            [NSCursor.resizeLeftRightCursor set];
            break;
        case 6:
            [NSCursor.resizeUpDownCursor set];
            break;
        default:
            [NSCursor.arrowCursor set];
            break;
        }
    }
}
