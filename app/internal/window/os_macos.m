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
	CFTypeRef view = (__bridge CFTypeRef)self.window.contentView;
	gio_updateDisplayLink(view, dispID);
}
- (void)windowDidBecomeKey:(NSNotification *)notification {
	gio_onFocus((__bridge CFTypeRef)self.window.contentView, YES);
}
- (void)windowDidResignKey:(NSNotification *)notification {
	gio_onFocus((__bridge CFTypeRef)self.window.contentView, NO);
}
- (void)windowWillClose:(NSNotification *)notification {
	gio_onTerminate((__bridge CFTypeRef)self.window.contentView);
	self.window.delegate = nil;
	[NSApp terminate:nil];
}
@end

CGFloat gio_viewHeight(CFTypeRef viewRef) {
	NSView *view = (__bridge NSView *)viewRef;
	return [view bounds].size.height;
}

CGFloat gio_viewWidth(CFTypeRef viewRef) {
	NSView *view = (__bridge NSView *)viewRef;
	return [view bounds].size.width;
}

CGFloat gio_getViewBackingScale(CFTypeRef viewRef) {
	NSView *view = (__bridge NSView *)viewRef;
	return [view.window backingScaleFactor];
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

		NSRect rect = NSMakeRect(0, 0, width, height);
		NSUInteger styleMask = NSTitledWindowMask |
			NSResizableWindowMask |
			NSMiniaturizableWindowMask |
			NSClosableWindowMask;
		NSWindow* window = [[NSWindow alloc] initWithContentRect:rect
													   styleMask:styleMask
														 backing:NSBackingStoreBuffered
														   defer:NO];
		window.title = [NSString stringWithUTF8String: title];
		[window cascadeTopLeftFromPoint:NSMakePoint(20,20)];
		[window setAcceptsMouseMovedEvents:YES];

		gio_onCreate((__bridge CFTypeRef)view);
		GioDelegate *del = [[GioDelegate alloc] init];
		del.window = window;
		[window setDelegate:del];
		[NSApp setDelegate:del];
		[window setContentView:view];
		[window makeFirstResponder:view];


		[NSApp run];
	}
}
