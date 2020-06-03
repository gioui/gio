// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,!ios

@import AppKit;

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
	gio_onChangeScreen(view, dispID);
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
		gio_onCreate((__bridge CFTypeRef)view);
		return (__bridge_retained CFTypeRef)window;
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

		NSWindow *window = (__bridge NSWindow *)gio_createWindow(viewRef, title, width, height);

		GioDelegate *del = [[GioDelegate alloc] init];
		del.window = window;
		[window setDelegate:del];
		[NSApp setDelegate:del];

		[NSApp run];
	}
}
