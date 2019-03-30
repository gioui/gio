// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,!ios

@import AppKit;

#include "os_macos.h"
#include "_cgo_export.h"

@interface GioDelegate : NSObject<NSApplicationDelegate, NSWindowDelegate>
@property (strong,nonatomic) NSWindow *window;
@end

@implementation GioDelegate
- (void)applicationDidFinishLaunching:(NSNotification *)aNotification {
    [[NSRunningApplication currentApplication] activateWithOptions:(NSApplicationActivateAllWindows | NSApplicationActivateIgnoringOtherApps)];
    [self.window makeKeyAndOrderFront:self];
	gio_onShow((__bridge CFTypeRef)self.window.contentView);
}
- (void)applicationDidHide:(NSNotification *)aNotification {
	gio_onHide((__bridge CFTypeRef)self.window.contentView);
}
- (void)applicationWillUnhide:(NSNotification *)notification {
	gio_onShow((__bridge CFTypeRef)self.window.contentView);
}
- (void)windowWillMiniaturize:(NSNotification *)notification {
	gio_onHide((__bridge CFTypeRef)self.window.contentView);
}
- (void)windowDidDeminiaturize:(NSNotification *)notification {
	gio_onShow((__bridge CFTypeRef)self.window.contentView);
}
- (void)windowDidChangeScreen:(NSNotification *)notification {
	CGDirectDisplayID dispID = [[[self.window screen] deviceDescription][@"NSScreenNumber"] unsignedIntValue];
	gio_updateDisplayLink((__bridge CFTypeRef)self.window.contentView, dispID);
}
- (void)windowWillClose:(NSNotification *)notification {
	gio_onTerminate((__bridge CFTypeRef)self.window.contentView);
	self.window.delegate = nil;
	[NSApp terminate:nil];
}
@end

CGFloat gio_viewHeight(CFTypeRef viewRef) {
	NSView *view = (__bridge NSView *)viewRef;
    NSRect bounds = [view convertRectToBacking:[view bounds]];
	return bounds.size.height;
}

CGFloat gio_viewWidth(CFTypeRef viewRef) {
	NSView *view = (__bridge NSView *)viewRef;
    NSRect bounds = [view convertRectToBacking:[view bounds]];
	return bounds.size.width;
}

// Points pr. dp.
static CGFloat getPointsPerDP(NSScreen *screen) {
	NSDictionary *description = [screen deviceDescription];
	NSSize displayPixelSize = [[description objectForKey:NSDeviceSize] sizeValue];
	CGSize displayPhysicalSize = CGDisplayScreenSize([[description objectForKey:@"NSScreenNumber"] unsignedIntValue]);
	return (25.4/160)*displayPixelSize.width / displayPhysicalSize.width;
}

// Pixels pr dp.
CGFloat gio_getPixelsPerDP() {
    NSScreen *screen = [NSScreen mainScreen];
    return [screen backingScaleFactor] * getPointsPerDP(screen);
}

void gio_main(CFTypeRef viewRef, const char *title, CGFloat width, CGFloat height) {
	@autoreleasepool {
		NSView *view = (NSView *)CFBridgingRelease(viewRef);
		[NSApplication sharedApplication];
		[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];

		NSMenuItem *mainMenu = [NSMenuItem new];

		NSMenu *menu = [NSMenu new];
		NSMenuItem *hideMenuItem = [[NSMenuItem alloc] initWithTitle:@"Hide"
															  action:@selector(hide:)
													   keyEquivalent:@"h"];
		[menu addItem:hideMenuItem];
		NSMenuItem *quitMenuItem = [[NSMenuItem alloc] initWithTitle:@"Quit"
															  action:@selector(terminate:)
													   keyEquivalent:@"q"];
		[menu addItem:quitMenuItem];
		[mainMenu setSubmenu:menu];
		NSMenu *menuBar = [NSMenu new];
		[menuBar addItem:mainMenu];
		[NSApp setMainMenu:menuBar];

		// Width and height are in pixels; convert to points
		CGFloat scale = [[NSScreen mainScreen] backingScaleFactor];
		width /= scale;
		height /= scale;

		NSRect rect = NSMakeRect(0, 0, width, height);
		NSWindowStyleMask styleMask = NSWindowStyleMaskTitled |
			NSWindowStyleMaskResizable |
			NSWindowStyleMaskMiniaturizable |
			NSWindowStyleMaskClosable;
		NSWindow* window = [[NSWindow alloc] initWithContentRect:rect
													   styleMask:styleMask
														 backing:NSBackingStoreBuffered
														   defer:NO];
		window.title = [NSString stringWithUTF8String: title];
		[window cascadeTopLeftFromPoint:NSMakePoint(20,20)];
		[window setAcceptsMouseMovedEvents:YES];

		[window setContentView:view];
		[window makeFirstResponder:view];

		GioDelegate *del = [[GioDelegate alloc] init];
		del.window = window;
		[window setDelegate:del];
		[NSApp setDelegate:del];
		gio_onCreate((__bridge CFTypeRef)view);

		[NSApp run];
	}
}
