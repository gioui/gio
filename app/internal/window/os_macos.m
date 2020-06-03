// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,!ios

@import AppKit;

#include "_cgo_export.h"

@interface GioAppDelegate : NSObject<NSApplicationDelegate>
@end

@interface GioWindowDelegate : NSObject<NSWindowDelegate>
@end

@implementation GioAppDelegate
- (void)applicationDidFinishLaunching:(NSNotification *)aNotification {
	[[NSRunningApplication currentApplication] activateWithOptions:(NSApplicationActivateAllWindows | NSApplicationActivateIgnoringOtherApps)];
}
- (void)applicationDidHide:(NSNotification *)aNotification {
	gio_onAppHide();
}
- (void)applicationWillUnhide:(NSNotification *)notification {
	gio_onAppShow();
}
@end

@implementation GioWindowDelegate
- (void)windowWillMiniaturize:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
	gio_onHide((__bridge CFTypeRef)window.contentView);
}
- (void)windowDidDeminiaturize:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
	gio_onShow((__bridge CFTypeRef)window.contentView);
}
- (void)windowDidChangeScreen:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
	CGDirectDisplayID dispID = [[[window screen] deviceDescription][@"NSScreenNumber"] unsignedIntValue];
	CFTypeRef view = (__bridge CFTypeRef)window.contentView;
	gio_onChangeScreen(view, dispID);
}
- (void)windowDidBecomeKey:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
	gio_onFocus((__bridge CFTypeRef)window.contentView, YES);
}
- (void)windowDidResignKey:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
	gio_onFocus((__bridge CFTypeRef)window.contentView, NO);
}
- (void)windowWillClose:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
	window.delegate = nil;
	gio_onClose((__bridge CFTypeRef)window.contentView);
}
@end

// Delegates are weakly referenced from their peers. Nothing
// else holds a strong reference to our window delegate, so
// keep a single global reference instead.
static GioWindowDelegate *globalWindowDel;

void gio_writeClipboard(unichar *chars, NSUInteger length) {
	@autoreleasepool {
		NSString *s = [NSString string];
		if (length > 0) {
			s = [NSString stringWithCharacters:chars length:length];
		}
		NSPasteboard *p = NSPasteboard.generalPasteboard;
		[p declareTypes:@[NSPasteboardTypeString] owner:nil];
		[p setString:s forType:NSPasteboardTypeString];
	}
}

CFTypeRef gio_readClipboard(void) {
	@autoreleasepool {
		NSPasteboard *p = NSPasteboard.generalPasteboard;
		NSString *content = [p stringForType:NSPasteboardTypeString];
		return (__bridge_retained CFTypeRef)content;
	}
}

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

static CVReturn displayLinkCallback(CVDisplayLinkRef dl, const CVTimeStamp *inNow, const CVTimeStamp *inOutputTime, CVOptionFlags flagsIn, CVOptionFlags *flagsOut, void *displayLinkContext) {
	gio_onFrameCallback(dl);
	return kCVReturnSuccess;
}

CFTypeRef gio_createDisplayLink(void) {
	CVDisplayLinkRef dl;
	CVDisplayLinkCreateWithActiveCGDisplays(&dl);
	CVDisplayLinkSetOutputCallback(dl, displayLinkCallback, nil);
	return dl;
}

int gio_startDisplayLink(CFTypeRef dl) {
	return CVDisplayLinkStart((CVDisplayLinkRef)dl);
}

int gio_stopDisplayLink(CFTypeRef dl) {
	return CVDisplayLinkStop((CVDisplayLinkRef)dl);
}

void gio_releaseDisplayLink(CFTypeRef dl) {
	CVDisplayLinkRelease((CVDisplayLinkRef)dl);
}

void gio_setDisplayLinkDisplay(CFTypeRef dl, uint64_t did) {
	CVDisplayLinkSetCurrentCGDisplay((CVDisplayLinkRef)dl, (CGDirectDisplayID)did);
}

CFTypeRef gio_createWindow(CFTypeRef viewRef, const char *title, CGFloat width, CGFloat height) {
	@autoreleasepool {
		NSRect rect = NSMakeRect(0, 0, width, height);
		NSUInteger styleMask = NSTitledWindowMask |
			NSResizableWindowMask |
			NSMiniaturizableWindowMask |
			NSClosableWindowMask;

		NSWindow* window = [[NSWindow alloc] initWithContentRect:rect
													   styleMask:styleMask
														 backing:NSBackingStoreBuffered
														   defer:NO];
		[window setAcceptsMouseMovedEvents:YES];
		window.title = [NSString stringWithUTF8String: title];
		NSView *view = (NSView *)CFBridgingRelease(viewRef);
		[window setContentView:view];
		[window makeFirstResponder:view];
		window.delegate = globalWindowDel;
		gio_onCreate((__bridge CFTypeRef)view);
		return (__bridge_retained CFTypeRef)window;
	}
}

void gio_makeKeyAndOrderFront(CFTypeRef viewRef) {
	NSView *view = (__bridge NSView *)viewRef;
	[view.window makeKeyAndOrderFront:nil];
}

void gio_appTerminate(void) {
	@autoreleasepool {
		[NSApp terminate:nil];
	}
}

void gio_main(CFTypeRef viewRef, const char *title, CGFloat width, CGFloat height) {
	@autoreleasepool {
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

		GioAppDelegate *del = [[GioAppDelegate alloc] init];

		globalWindowDel = [[GioWindowDelegate alloc] init];
		NSWindow *window = (__bridge NSWindow *)gio_createWindow(viewRef, title, width, height);

		[NSApp setDelegate:del];

		[NSApp run];
	}
}
