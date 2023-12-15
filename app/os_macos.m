// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,!ios

#import <AppKit/AppKit.h>

#include "_cgo_export.h"

__attribute__ ((visibility ("hidden"))) CALayer *gio_layerFactory(void);

@interface GioAppDelegate : NSObject<NSApplicationDelegate>
@end

@interface GioWindowDelegate : NSObject<NSWindowDelegate>
@end

@interface GioView : NSView <CALayerDelegate,NSTextInputClient>
@property uintptr_t handle;
@end

@implementation GioWindowDelegate
- (void)windowWillMiniaturize:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
  GioView *view = (GioView *)window.contentView;
	gio_onHide(view.handle);
}
- (void)windowDidDeminiaturize:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
  GioView *view = (GioView *)window.contentView;
	gio_onShow(view.handle);
}
- (void)windowWillEnterFullScreen:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
  GioView *view = (GioView *)window.contentView;
	gio_onFullscreen(view.handle);
}
- (void)windowWillExitFullScreen:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
  GioView *view = (GioView *)window.contentView;
	gio_onWindowed(view.handle);
}
- (void)windowDidChangeScreen:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
	CGDirectDisplayID dispID = [[[window screen] deviceDescription][@"NSScreenNumber"] unsignedIntValue];
  GioView *view = (GioView *)window.contentView;
	gio_onChangeScreen(view.handle, dispID);
}
- (void)windowDidBecomeKey:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
  GioView *view = (GioView *)window.contentView;
	gio_onFocus(view.handle, 1);
}
- (void)windowDidResignKey:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
  GioView *view = (GioView *)window.contentView;
	gio_onFocus(view.handle, 0);
}
@end

static void handleMouse(GioView *view, NSEvent *event, int typ, CGFloat dx, CGFloat dy) {
	NSPoint p = [view convertPoint:[event locationInWindow] fromView:nil];
	if (!event.hasPreciseScrollingDeltas) {
		// dx and dy are in rows and columns.
		dx *= 10;
		dy *= 10;
	}
	// Origin is in the lower left corner. Convert to upper left.
	CGFloat height = view.bounds.size.height;
	gio_onMouse(view.handle, (__bridge CFTypeRef)event, typ, event.buttonNumber, p.x, height - p.y, dx, dy, [event timestamp], [event modifierFlags]);
}

@interface GioApplication: NSApplication
@end

// Variables for tracking resizes.
static struct {
	NSPoint dir;
	NSEvent *lastMouseDown;
	NSPoint off;
} resizeState = {};

static NSBitmapImageRep *nsImageBitmap(NSImage *img) {
	NSArray<NSImageRep *> *reps = img.representations;
	if ([reps count] == 0) {
		return nil;
	}
	NSImageRep *rep = reps[0];
	if (![rep isKindOfClass:[NSBitmapImageRep class]]) {
		return nil;
	}
	return (NSBitmapImageRep *)rep;
}

static NSCursor *lookupPrivateNSCursor(SEL name) {
	if (![NSCursor respondsToSelector:name]) {
		return nil;
	}
	id obj = [NSCursor performSelector:name];
	if (![obj isKindOfClass:[NSCursor class]]) {
		return nil;
	}
	return (NSCursor *)obj;
}

static BOOL isEqualNSCursor(NSCursor *c1, SEL name2) {
	NSCursor *c2 = lookupPrivateNSCursor(name2);
	if (c2 == nil || !NSEqualPoints(c1.hotSpot, c2.hotSpot)) {
		return NO;
	}
	NSImage *img1 = c1.image;
	NSImage *img2 = c2.image;
	if (!NSEqualSizes(img1.size, img2.size)) {
		return NO;
	}
	NSBitmapImageRep *bit1 = nsImageBitmap(img1);
	NSBitmapImageRep *bit2 = nsImageBitmap(img2);
	if (bit1 == nil || bit2 == nil) {
		return NO;
	}
	NSInteger n1 = bit1.numberOfPlanes*bit1.bytesPerPlane;
	NSInteger n2 = bit1.numberOfPlanes*bit1.bytesPerPlane;
	if (n1 != n2) {
		return NO;
	}
	if (memcmp(bit1.bitmapData, bit2.bitmapData, n1) != 0) {
		return NO;
	}
	return YES;
}

@implementation GioApplication
- (NSEvent *)nextEventMatchingMask:(NSEventMask)mask
                         untilDate:(NSDate *)expiration
                            inMode:(NSRunLoopMode)mode
                           dequeue:(BOOL)deqFlag {
	if ([mode isEqualToString:NSEventTrackingRunLoopMode]) {
		NSEvent *l = resizeState.lastMouseDown;
		if (l != nil) {
			//lastMouseDown = nil;
			NSCursor *cur = [NSCursor currentSystemCursor];
			NSPoint dir = {};
			NSPoint off = {};
			NSSize wsz = [l window].frame.size;
			NSPoint center = NSMakePoint(wsz.width/2, wsz.height/2);
			NSPoint p = [l locationInWindow];
			if (p.x >= center.x) {
				dir.x = 1;
				off.x = p.x - wsz.width;
			} else {
				dir.x = -1;
				off.x = p.x;
			}
			if (p.y >= center.y) {
				dir.y = 1;
				off.y = p.y - wsz.height;
			} else {
				dir.y = -1;
				off.y = p.y;
			}
			// The button down coordinate distinguish the four quadrants. Use the
			// cursor image to determine the precise direction.
			SEL nw = @selector(_windowResizeNorthWestCursor);
			SEL n = @selector(_windowResizeNorthCursor);
			SEL ne = @selector(_windowResizeNorthEastCursor);
			SEL e = @selector(_windowResizeEastCursor);
			SEL se = @selector(_windowResizeSouthEastCursor);
			SEL s = @selector(_windowResizeSouthCursor);
			SEL sw = @selector(_windowResizeSouthWestCursor);
			SEL w = @selector(_windowResizeWestCursor);
			SEL ns = @selector(_windowResizeNorthSouthCursor);
			SEL ew = @selector(_windowResizeEastWestCursor);
			SEL nwse = @selector(_windowResizeNorthWestSouthEastCursor);
			SEL nesw = @selector(_windowResizeNorthEastSouthWestCursor);
			BOOL match = YES;
			if (dir.x != 0 && (isEqualNSCursor(cur, ew) || isEqualNSCursor(cur, w) || isEqualNSCursor(cur, e))) {
				dir.y = 0;
			}
			if (dir.y != 0 && (isEqualNSCursor(cur, ns) || isEqualNSCursor(cur, s) || isEqualNSCursor(cur, n))) {
					dir.x = 0;
			}
			// If none of the cursors matched, we may deduce that the resize
			// direction is one of the corners. However, to ensure that at least
			// one cursor matches, check the corner cursors.
			if (dir.x == 1 && dir.y == 1) {
				if (!isEqualNSCursor(cur, nesw) && !isEqualNSCursor(cur, sw)) {
					dir = NSZeroPoint;
				}
			} else if (dir.x == 1 && dir.y == -1) {
				if (!isEqualNSCursor(cur, nwse) && !isEqualNSCursor(cur, nw)) {
					dir = NSZeroPoint;
				}
			} else if (dir.x == -1 && dir.y == 1) {
				if (!isEqualNSCursor(cur, nwse) && !isEqualNSCursor(cur, se)) {
					dir = NSZeroPoint;
				}
			} else if (dir.x == -1 && dir.y == -1) {
				if (!isEqualNSCursor(cur, nesw) && !isEqualNSCursor(cur, ne)) {
					dir = NSZeroPoint;
				}
			}
			if (!NSEqualPoints(dir, NSZeroPoint)) {
				NSEvent *cancel = [NSEvent mouseEventWithType:NSEventTypeLeftMouseUp
											                       location:l.locationInWindow
											                  modifierFlags:l.modifierFlags
											                      timestamp:l.timestamp
											                   windowNumber:l.windowNumber
											                        context:l.context
											                    eventNumber:l.eventNumber
											                     clickCount:l.clickCount
											                       pressure:l.pressure];
				resizeState.off = off;
				resizeState.dir = dir;
				return cancel;
			}
		}
	}
	return [super nextEventMatchingMask:mask untilDate:expiration inMode:mode dequeue:deqFlag];
}
@end

@interface GioWindow: NSWindow
@end

@implementation GioWindow
- (void)sendEvent:(NSEvent *)evt {
	if (evt.type == NSEventTypeLeftMouseDown) {
		resizeState.lastMouseDown = evt;
	}
	NSPoint dir = resizeState.dir;
	if (NSEqualPoints(dir, NSZeroPoint)) {
		[super sendEvent:evt];
		return;
	}
	switch (evt.type) {
	default:
		return;
	case NSEventTypeLeftMouseUp:
		resizeState.dir = NSZeroPoint;
		resizeState.lastMouseDown = nil;
		return;
	case NSEventTypeLeftMouseDragged:
		// Ok to proceed.
		break;
	}
	NSPoint loc = evt.locationInWindow;
	NSPoint off = resizeState.off;
	loc.x -= off.x;
	loc.y -= off.y;
	NSRect frame = [self frame];
	NSSize min = [self minSize];
	NSSize max = [self maxSize];
	CGFloat width = frame.size.width;
	if (dir.x > 0) {
		width = loc.x;
	} else if (dir.x < 0) {
		width -= loc.x;
	}
	width = MIN(max.width, MAX(min.width, width));
	if (dir.x < 0) {
		frame.origin.x += frame.size.width - width;
	}
	frame.size.width = width;
	CGFloat height = frame.size.height;
	if (dir.y > 0) {
		height = loc.y;
	} else if (dir.y < 0) {
		height -= loc.y;
	}
	height = MIN(max.height, MAX(min.height, height));
	if (dir.y < 0) {
		frame.origin.y += frame.size.height - height;
	}
	frame.size.height = height;
	[self setFrame:frame display:YES animate:NO];
}
@end

@implementation GioView
- (void)setFrameSize:(NSSize)newSize {
	[super setFrameSize:newSize];
	[self setNeedsDisplay:YES];
}
// drawRect is called when OpenGL is used, displayLayer otherwise.
// Don't know why.
- (void)drawRect:(NSRect)r {
	gio_onDraw(self.handle);
}
- (void)displayLayer:(CALayer *)layer {
	layer.contentsScale = self.window.backingScaleFactor;
	gio_onDraw(self.handle);
}
- (CALayer *)makeBackingLayer {
	CALayer *layer = gio_layerFactory();
	layer.delegate = self;
	return layer;
}
- (void)viewDidMoveToWindow {
	gio_onAttached(self.handle, self.window != nil ? 1 : 0);
}
- (void)mouseDown:(NSEvent *)event {
	handleMouse(self, event, MOUSE_DOWN, 0, 0);
}
- (void)mouseUp:(NSEvent *)event {
	handleMouse(self, event, MOUSE_UP, 0, 0);
}
- (void)rightMouseDown:(NSEvent *)event {
	handleMouse(self, event, MOUSE_DOWN, 0, 0);
}
- (void)rightMouseUp:(NSEvent *)event {
	handleMouse(self, event, MOUSE_UP, 0, 0);
}
- (void)otherMouseDown:(NSEvent *)event {
	handleMouse(self, event, MOUSE_DOWN, 0, 0);
}
- (void)otherMouseUp:(NSEvent *)event {
	handleMouse(self, event, MOUSE_UP, 0, 0);
}
- (void)mouseMoved:(NSEvent *)event {
	handleMouse(self, event, MOUSE_MOVE, 0, 0);
}
- (void)mouseDragged:(NSEvent *)event {
	handleMouse(self, event, MOUSE_MOVE, 0, 0);
}
- (void)rightMouseDragged:(NSEvent *)event {
	handleMouse(self, event, MOUSE_MOVE, 0, 0);
}
- (void)otherMouseDragged:(NSEvent *)event {
	handleMouse(self, event, MOUSE_MOVE, 0, 0);
}
- (void)scrollWheel:(NSEvent *)event {
	CGFloat dx = -event.scrollingDeltaX;
	CGFloat dy = -event.scrollingDeltaY;
	handleMouse(self, event, MOUSE_SCROLL, dx, dy);
}
- (void)keyDown:(NSEvent *)event {
	[self interpretKeyEvents:[NSArray arrayWithObject:event]];
	NSString *keys = [event charactersIgnoringModifiers];
	gio_onKeys(self.handle, (__bridge CFTypeRef)keys, [event timestamp], [event modifierFlags], true);
}
- (void)keyUp:(NSEvent *)event {
	NSString *keys = [event charactersIgnoringModifiers];
	gio_onKeys(self.handle, (__bridge CFTypeRef)keys, [event timestamp], [event modifierFlags], false);
}
- (void)insertText:(id)string {
	gio_onText(self.handle, (__bridge CFTypeRef)string);
}
- (void)doCommandBySelector:(SEL)sel {
	// Don't pass commands up the responder chain.
	// They will end up in a beep.
}

- (BOOL)hasMarkedText {
	int res = gio_hasMarkedText(self.handle);
	return res ? YES : NO;
}
- (NSRange)markedRange {
	return gio_markedRange(self.handle);
}
- (NSRange)selectedRange {
	return gio_selectedRange(self.handle);
}
- (void)unmarkText {
	gio_unmarkText(self.handle);
}
- (void)setMarkedText:(id)string
        selectedRange:(NSRange)selRange
     replacementRange:(NSRange)replaceRange {
	NSString *str;
	// string is either an NSAttributedString or an NSString.
	if ([string isKindOfClass:[NSAttributedString class]]) {
		str = [string string];
	} else {
		str = string;
	}
	gio_setMarkedText(self.handle, (__bridge CFTypeRef)str, selRange, replaceRange);
}
- (NSArray<NSAttributedStringKey> *)validAttributesForMarkedText {
	return nil;
}
- (NSAttributedString *)attributedSubstringForProposedRange:(NSRange)range
                                                actualRange:(NSRangePointer)actualRange {
	NSString *str = CFBridgingRelease(gio_substringForProposedRange(self.handle, range, actualRange));
	return [[NSAttributedString alloc] initWithString:str attributes:nil];
}
- (void)insertText:(id)string
  replacementRange:(NSRange)replaceRange {
	NSString *str;
	// string is either an NSAttributedString or an NSString.
	if ([string isKindOfClass:[NSAttributedString class]]) {
		str = [string string];
	} else {
		str = string;
	}
	gio_insertText(self.handle, (__bridge CFTypeRef)str, replaceRange);
}
- (NSUInteger)characterIndexForPoint:(NSPoint)p {
	return gio_characterIndexForPoint(self.handle, p);
}
- (NSRect)firstRectForCharacterRange:(NSRange)rng
                         actualRange:(NSRangePointer)actual {
    NSRect r = gio_firstRectForCharacterRange(self.handle, rng, actual);
    r = [self convertRect:r toView:nil];
    return [[self window] convertRectToScreen:r];
}
- (void)applicationWillUnhide:(NSNotification *)notification {
	gio_onShow(self.handle);
}
- (void)applicationDidHide:(NSNotification *)notification {
	gio_onHide(self.handle);
}
- (void)dealloc {
	gio_onDestroy(self.handle);
}
@end

// Delegates are weakly referenced from their peers. Nothing
// else holds a strong reference to our window delegate, so
// keep a single global reference instead.
static GioWindowDelegate *globalWindowDel;

static CVReturn displayLinkCallback(CVDisplayLinkRef dl, const CVTimeStamp *inNow, const CVTimeStamp *inOutputTime, CVOptionFlags flagsIn, CVOptionFlags *flagsOut, void *handle) {
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

// some cursors are not public, this tries to use a private cursor
// and uses fallback when the use of private cursor fails.
static void trySetPrivateCursor(SEL cursorName, NSCursor* fallback) {
	NSCursor *cur = lookupPrivateNSCursor(cursorName);
	if (cur == nil) {
		cur = fallback;
	}
	[cur set];
}

void gio_setCursor(NSUInteger curID) {
	@autoreleasepool {
		switch (curID) {
			case 0: // pointer.CursorDefault
				[NSCursor.arrowCursor set];
				break;
			// case 1: // pointer.CursorNone
			case 2: // pointer.CursorText
				[NSCursor.IBeamCursor set];
				break;
			case 3: // pointer.CursorVerticalText
				[NSCursor.IBeamCursorForVerticalLayout set];
				break;
			case 4: // pointer.CursorPointer
				[NSCursor.pointingHandCursor set];
				break;
			case 5: // pointer.CursorCrosshair
				[NSCursor.crosshairCursor set];
				break;
			case 6: // pointer.CursorAllScroll
				// For some reason, using _moveCursor fails on Monterey.
				// trySetPrivateCursor(@selector(_moveCursor), NSCursor.arrowCursor);
				[NSCursor.arrowCursor set];
				break;
			case 7: // pointer.CursorColResize
				[NSCursor.resizeLeftRightCursor set];
				break;
			case 8: // pointer.CursorRowResize
				[NSCursor.resizeUpDownCursor set];
				break;
			case 9: // pointer.CursorGrab
				[NSCursor.openHandCursor set];
				break;
			case 10: // pointer.CursorGrabbing
				[NSCursor.closedHandCursor set];
				break;
			case 11: // pointer.CursorNotAllowed
				[NSCursor.operationNotAllowedCursor set];
				break;
			case 12: // pointer.CursorWait
				trySetPrivateCursor(@selector(busyButClickableCursor), NSCursor.arrowCursor);
				break;
			case 13: // pointer.CursorProgress
				trySetPrivateCursor(@selector(busyButClickableCursor), NSCursor.arrowCursor);
				break;
			case 14: // pointer.CursorNorthWestResize
				trySetPrivateCursor(@selector(_windowResizeNorthWestCursor), NSCursor.resizeUpDownCursor);
				break;
			case 15: // pointer.CursorNorthEastResize
				trySetPrivateCursor(@selector(_windowResizeNorthEastCursor), NSCursor.resizeUpDownCursor);
				break;
			case 16: // pointer.CursorSouthWestResize
				trySetPrivateCursor(@selector(_windowResizeSouthWestCursor), NSCursor.resizeUpDownCursor);
				break;
			case 17: // pointer.CursorSouthEastResize
				trySetPrivateCursor(@selector(_windowResizeSouthEastCursor), NSCursor.resizeUpDownCursor);
				break;
			case 18: // pointer.CursorNorthSouthResize
				[NSCursor.resizeUpDownCursor set];
				break;
			case 19: // pointer.CursorEastWestResize
				[NSCursor.resizeLeftRightCursor set];
				break;
			case 20: // pointer.CursorWestResize
				[NSCursor.resizeLeftCursor set];
				break;
			case 21: // pointer.CursorEastResize
				[NSCursor.resizeRightCursor set];
				break;
			case 22: // pointer.CursorNorthResize
				[NSCursor.resizeUpCursor set];
				break;
			case 23: // pointer.CursorSouthResize
				[NSCursor.resizeDownCursor set];
				break;
			case 24: // pointer.CursorNorthEastSouthWestResize
				trySetPrivateCursor(@selector(_windowResizeNorthEastSouthWestCursor), NSCursor.resizeUpDownCursor);
				break;
			case 25: // pointer.CursorNorthWestSouthEastResize
				trySetPrivateCursor(@selector(_windowResizeNorthWestSouthEastCursor), NSCursor.resizeUpDownCursor);
				break;
			default:
				[NSCursor.arrowCursor set];
				break;
		}
	}
}

CFTypeRef gio_createWindow(CFTypeRef viewRef, CGFloat width, CGFloat height, CGFloat minWidth, CGFloat minHeight, CGFloat maxWidth, CGFloat maxHeight) {
	@autoreleasepool {
		NSRect rect = NSMakeRect(0, 0, width, height);
		NSUInteger styleMask = NSTitledWindowMask |
			NSResizableWindowMask |
			NSMiniaturizableWindowMask |
			NSClosableWindowMask;

		GioWindow* window = [[GioWindow alloc] initWithContentRect:rect
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
		NSView *view = (__bridge NSView *)viewRef;
		[window setContentView:view];
		[window makeFirstResponder:view];
		window.delegate = globalWindowDel;
		return (__bridge_retained CFTypeRef)window;
	}
}

CFTypeRef gio_createView(void) {
	@autoreleasepool {
		NSRect frame = NSMakeRect(0, 0, 0, 0);
		GioView* view = [[GioView alloc] initWithFrame:frame];
		view.wantsLayer = YES;
		view.layerContentsRedrawPolicy = NSViewLayerContentsRedrawDuringViewResize;

		[[NSNotificationCenter defaultCenter] addObserver:view
												 selector:@selector(applicationWillUnhide:)
													 name:NSApplicationWillUnhideNotification
												   object:nil];
		[[NSNotificationCenter defaultCenter] addObserver:view
												 selector:@selector(applicationDidHide:)
													 name:NSApplicationDidHideNotification
												   object:nil];
		return CFBridgingRetain(view);
	}
}

void gio_viewSetHandle(CFTypeRef viewRef, uintptr_t handle) {
	@autoreleasepool {
		GioView *v = (__bridge GioView *)viewRef;
		v.handle = handle;
	}
}

@implementation GioAppDelegate
- (void)applicationDidFinishLaunching:(NSNotification *)aNotification {
	[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
	[NSApp activateIgnoringOtherApps:YES];
	gio_onFinishLaunching();
}
@end

void gio_main() {
	@autoreleasepool {
		[GioApplication sharedApplication];
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
