// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,!ios

@import AppKit;

#include "_cgo_export.h"

@interface GioAppDelegate : NSObject<NSApplicationDelegate>
@end

@interface GioWindowDelegate : NSObject<NSWindowDelegate>
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
	gio_onFocus((__bridge CFTypeRef)window.contentView, 1);
}
- (void)windowDidResignKey:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
	gio_onFocus((__bridge CFTypeRef)window.contentView, 0);
}
- (void)windowWillClose:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
	window.delegate = nil;
	gio_onClose((__bridge CFTypeRef)window.contentView);
}
@end

static void handleMouse(NSView *view, NSEvent *event, int typ, CGFloat dx, CGFloat dy) {
	NSPoint p = [view convertPoint:[event locationInWindow] fromView:nil];
	if (!event.hasPreciseScrollingDeltas) {
		// dx and dy are in rows and columns.
		dx *= 10;
		dy *= 10;
	}
	// Origin is in the lower left corner. Convert to upper left.
	CGFloat height = view.bounds.size.height;
	gio_onMouse((__bridge CFTypeRef)view, typ, [NSEvent pressedMouseButtons], p.x, height - p.y, dx, dy, [event timestamp], [event modifierFlags]);
}

@interface GioView : NSView
@end

@implementation GioView
- (void)drawRect:(NSRect)r {
	gio_onDraw((__bridge CFTypeRef)self);
}
- (void)mouseDown:(NSEvent *)event {
	handleMouse(self, event, GIO_MOUSE_DOWN, 0, 0);
}
- (void)mouseUp:(NSEvent *)event {
	handleMouse(self, event, GIO_MOUSE_UP, 0, 0);
}
- (void)middleMouseDown:(NSEvent *)event {
	handleMouse(self, event, GIO_MOUSE_DOWN, 0, 0);
}
- (void)middletMouseUp:(NSEvent *)event {
	handleMouse(self, event, GIO_MOUSE_UP, 0, 0);
}
- (void)rightMouseDown:(NSEvent *)event {
	handleMouse(self, event, GIO_MOUSE_DOWN, 0, 0);
}
- (void)rightMouseUp:(NSEvent *)event {
	handleMouse(self, event, GIO_MOUSE_UP, 0, 0);
}
- (void)mouseMoved:(NSEvent *)event {
	handleMouse(self, event, GIO_MOUSE_MOVE, 0, 0);
}
- (void)mouseDragged:(NSEvent *)event {
	handleMouse(self, event, GIO_MOUSE_MOVE, 0, 0);
}
- (void)scrollWheel:(NSEvent *)event {
	CGFloat dx = -event.scrollingDeltaX;
	CGFloat dy = -event.scrollingDeltaY;
	handleMouse(self, event, GIO_MOUSE_SCROLL, dx, dy);
}
- (void)keyDown:(NSEvent *)event {
	NSString *keys = [event charactersIgnoringModifiers];
	gio_onKeys((__bridge CFTypeRef)self, (char *)[keys UTF8String], [event timestamp], [event modifierFlags], true);
	[self interpretKeyEvents:[NSArray arrayWithObject:event]];
}
- (void)keyUp:(NSEvent *)event {
	NSString *keys = [event charactersIgnoringModifiers];
	gio_onKeys((__bridge CFTypeRef)self, (char *)[keys UTF8String], [event timestamp], [event modifierFlags], false);
}
- (void)insertText:(id)string {
	const char *utf8 = [string UTF8String];
	gio_onText((__bridge CFTypeRef)self, (char *)utf8);
}
- (void)doCommandBySelector:(SEL)sel {
	// Don't pass commands up the responder chain.
	// They will end up in a beep.
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

CGFloat gio_getScreenBackingScale(void) {
	return [NSScreen.mainScreen backingScaleFactor];
}

CGFloat gio_getViewBackingScale(CFTypeRef viewRef) {
	NSView *view = (__bridge NSView *)viewRef;
	return [view.window backingScaleFactor];
}

void gio_setNeedsDisplay(CFTypeRef viewRef) {
	NSView *view = (__bridge NSView *)viewRef;
	[view setNeedsDisplay:YES];
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
			case 7:
				[NSCursor.openHandCursor set];
				break;
			default:
				[NSCursor.arrowCursor set];
				break;
		}
	}
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

NSPoint gio_cascadeTopLeftFromPoint(CFTypeRef windowRef, NSPoint topLeft) {
	NSWindow *window = (__bridge NSWindow *)windowRef;
	return [window cascadeTopLeftFromPoint:topLeft];
}

void gio_makeKeyAndOrderFront(CFTypeRef windowRef) {
	NSWindow *window = (__bridge NSWindow *)windowRef;
	[window makeKeyAndOrderFront:nil];
}

void gio_toggleFullScreen(CFTypeRef windowRef) {
	NSWindow *window = (__bridge NSWindow *)windowRef;
	[window toggleFullScreen:nil];
}

CFTypeRef gio_createWindow(CFTypeRef viewRef, const char *title, CGFloat width, CGFloat height, CGFloat minWidth, CGFloat minHeight, CGFloat maxWidth, CGFloat maxHeight) {
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
		if (minWidth > 0 || minHeight > 0) {
			window.contentMinSize = NSMakeSize(minWidth, minHeight);
		}
		if (maxWidth > 0 || maxHeight > 0) {
			window.contentMaxSize = NSMakeSize(maxWidth, maxHeight);
		}
		[window setAcceptsMouseMovedEvents:YES];
		if (title != nil) {
			window.title = [NSString stringWithUTF8String: title];
		}
		NSView *view = (__bridge NSView *)viewRef;
		[window setContentView:view];
		[window makeFirstResponder:view];
		window.releasedWhenClosed = NO;
		window.delegate = globalWindowDel;
		return (__bridge_retained CFTypeRef)window;
	}
}

void gio_close(CFTypeRef windowRef) {
	NSWindow* window = (__bridge NSWindow *)windowRef;
	[window performClose:nil];
}

void gio_setSize(CFTypeRef windowRef, CGFloat width, CGFloat height) {
	NSWindow* window = (__bridge NSWindow *)windowRef;
	NSSize size = NSMakeSize(width, height);
	[window setContentSize:size];
}

void gio_setMinSize(CFTypeRef windowRef, CGFloat width, CGFloat height) {
	NSWindow* window = (__bridge NSWindow *)windowRef;
	window.contentMinSize = NSMakeSize(width, height);
}

void gio_setMaxSize(CFTypeRef windowRef, CGFloat width, CGFloat height) {
	NSWindow* window = (__bridge NSWindow *)windowRef;
	window.contentMaxSize = NSMakeSize(width, height);
}

void gio_setTitle(CFTypeRef windowRef, const char *title) {
	NSWindow* window = (__bridge NSWindow *)windowRef;
	window.title = [NSString stringWithUTF8String: title];
}

CFTypeRef gio_layerForView(CFTypeRef viewRef) {
	NSView *view = (__bridge NSView *)viewRef;
	return (__bridge CFTypeRef)view.layer;
}

CFTypeRef gio_createView(void) {
	@autoreleasepool {
		NSRect frame = NSMakeRect(0, 0, 0, 0);
		GioView* view = [[GioView alloc] initWithFrame:frame];
		[view setWantsLayer:YES];
		return CFBridgingRetain(view);
	}
}

@implementation GioAppDelegate
- (void)applicationDidFinishLaunching:(NSNotification *)aNotification {
	[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
	[NSApp activateIgnoringOtherApps:YES];
	gio_onFinishLaunching();
}
- (void)applicationDidHide:(NSNotification *)aNotification {
	gio_onAppHide();
}
- (void)applicationWillUnhide:(NSNotification *)notification {
	gio_onAppShow();
}
@end

void gio_main() {
	@autoreleasepool {
		[NSApplication sharedApplication];
		GioAppDelegate *del = [[GioAppDelegate alloc] init];
		[NSApp setDelegate:del];

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

		globalWindowDel = [[GioWindowDelegate alloc] init];

		[NSApp run];
	}
}
