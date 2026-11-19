// SPDX-License-Identifier: Unlicense OR MIT

//go:build darwin && !ios

#import <AppKit/AppKit.h>

#include "_cgo_export.h"

__attribute__ ((visibility ("hidden"))) CALayer *gio_layerFactory(BOOL presentWithTrans);

@interface GioAppDelegate : NSObject<NSApplicationDelegate>
@end

@interface GioWindowDelegate : NSObject<NSWindowDelegate>
@end

@interface GioView : NSView <CALayerDelegate,NSTextInputClient,NSDraggingDestination,NSDraggingSource>
@property uintptr_t handle;
@property BOOL presentWithTrans;
@property (copy) NSArray<NSString *> *dropTypes;
@property (strong) NSEvent *dragEvent;
- (NSString *)dropTypeFor:(id<NSDraggingInfo>)sender;
@end

static NSPasteboardType pasteboardType(NSString *mime) {
	if ([mime isEqualToString:@"text/plain"]) return NSPasteboardTypeString;
	if ([mime isEqualToString:@"image/png"]) return NSPasteboardTypePNG;
	if ([mime isEqualToString:@"text/html"]) return NSPasteboardTypeHTML;
	return [@"org.gioui.mime." stringByAppendingString:mime];
}

void gio_setDropTypes(CFTypeRef viewRef, CFTypeRef typesRef) {
	GioView *view = (__bridge GioView *)viewRef;
	view.dropTypes = (__bridge NSArray *)typesRef;
	NSMutableArray *types = [NSMutableArray array];
	for (NSString *mime in view.dropTypes) {
		[types addObject:pasteboardType(mime)];
		if ([mime isEqualToString:@"text/uri-list"]) [types addObjectsFromArray:@[NSPasteboardTypeFileURL, NSFilenamesPboardType]];
	}
	[view unregisterDraggedTypes];
	if (types.count) [view registerForDraggedTypes:types];
}

void gio_startDrag(CFTypeRef viewRef, CFTypeRef dataRef) {
	GioView *view = (__bridge GioView *)viewRef;
	NSDictionary<NSString *, NSData *> *data = (__bridge NSDictionary *)dataRef;
	NSEvent *event = view.dragEvent;
	dispatch_async(dispatch_get_main_queue(), ^{
		if (!view.window || !event || !view.dragEvent || !(NSEvent.pressedMouseButtons & 1)) {
			if (view.window) gio_onDragEnded(view.handle);
			return;
		}
		NSMutableArray<NSDraggingItem *> *items = [NSMutableArray array];
		NSPasteboardItem *item = [NSPasteboardItem new];
		for (NSString *mime in data) [item setData:data[mime] forType:pasteboardType(mime)];
		[items addObject:[[NSDraggingItem alloc] initWithPasteboardWriter:item]];
		NSString *uris = [[NSString alloc] initWithData:data[@"text/uri-list"] ?: [NSData data] encoding:NSUTF8StringEncoding];
		BOOL first = YES;
		for (NSString *line in [uris componentsSeparatedByCharactersInSet:NSCharacterSet.newlineCharacterSet]) {
			NSURL *url = [NSURL URLWithString:line];
			if (!url.isFileURL) continue;
			if (first) { [item setString:url.absoluteString forType:NSPasteboardTypeFileURL]; first = NO; }
			else [items addObject:[[NSDraggingItem alloc] initWithPasteboardWriter:url]];
		}
		NSPoint point = [view convertPoint:event.locationInWindow fromView:nil];
		NSImage *preview = [NSImage imageNamed:NSImageNameMultipleDocuments];
		for (NSDraggingItem *dragItem in items) [dragItem setDraggingFrame:NSMakeRect(point.x, point.y, 24, 24) contents:preview];
		[view beginDraggingSessionWithItems:items event:event source:view];
	});
}

@implementation GioWindowDelegate
- (BOOL)windowShouldClose:(NSWindow *)window {
	GioView *view = (GioView *)window.contentView;
	return gio_onCloseRequest(view.handle) != 0;
}
- (void)windowWillMiniaturize:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
  GioView *view = (GioView *)window.contentView;
	gio_onDraw(view.handle);
}
- (void)windowDidDeminiaturize:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
  GioView *view = (GioView *)window.contentView;
	gio_onDraw(view.handle);
}
- (void)windowWillEnterFullScreen:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
  GioView *view = (GioView *)window.contentView;
	gio_onDraw(view.handle);
}
- (void)windowWillExitFullScreen:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
  GioView *view = (GioView *)window.contentView;
	gio_onDraw(view.handle);
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
	if ([window firstResponder] == view) {
		gio_onFocus(view.handle, 1);
	}
}
- (void)windowDidResignKey:(NSNotification *)notification {
	NSWindow *window = (NSWindow *)[notification object];
	GioView *view = (GioView *)window.contentView;
	if ([window firstResponder] == view) {
		gio_onFocus(view.handle, 0);
	}
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

@implementation GioView
- (NSDragOperation)draggingSession:(NSDraggingSession *)session sourceOperationMaskForDraggingContext:(NSDraggingContext)context {
	return NSDragOperationCopy;
}
- (void)draggingSession:(NSDraggingSession *)session endedAtPoint:(NSPoint)point operation:(NSDragOperation)operation {
	self.dragEvent = nil;
	if (self.window) gio_onDragEnded(self.handle);
}
- (NSDragOperation)draggingEntered:(id<NSDraggingInfo>)sender {
	return [self draggingUpdated:sender];
}

- (NSDragOperation)draggingUpdated:(id<NSDraggingInfo>)sender {
	return [self dropTypeFor:sender] != nil ? NSDragOperationCopy : NSDragOperationNone;
}

- (NSString *)dropTypeFor:(id<NSDraggingInfo>)sender {
	NSPasteboard *pboard = [sender draggingPasteboard];
	NSMutableArray *offered = [NSMutableArray array];
	for (NSString *mime in self.dropTypes) {
		if ([pboard.types containsObject:pasteboardType(mime)] ||
		    ([mime isEqualToString:@"text/uri-list"] && ([pboard.types containsObject:NSPasteboardTypeFileURL] || [pboard.types containsObject:NSFilenamesPboardType]))) {
			[offered addObject:mime];
		}
	}
	NSPoint point = [self convertPoint:sender.draggingLocation fromView:nil];
	NSInteger index = gio_onPlatformDrag(self.handle, (__bridge CFTypeRef)offered, offered.count, point.x, self.bounds.size.height - point.y);
	return index >= 0 && (sender.draggingSourceOperationMask & NSDragOperationCopy) ? offered[(NSUInteger)index] : nil;
}

- (void)draggingExited:(id<NSDraggingInfo>)sender {
	if (self.window != nil) {
		gio_onPlatformDragExited(self.handle);
	}
}

- (void)draggingEnded:(id<NSDraggingInfo>)sender {
	[self draggingExited:sender];
}

- (BOOL)prepareForDragOperation:(id<NSDraggingInfo>)sender {
	return [self draggingEntered:sender] != NSDragOperationNone;
}

- (BOOL)performDragOperation:(id<NSDraggingInfo>)sender {
	NSPoint point = [self convertPoint:sender.draggingLocation fromView:nil];
	NSPasteboard *pboard = [sender draggingPasteboard];
	NSString *mime = [self dropTypeFor:sender];
	if (!mime) return NO;
	NSData *data = [pboard dataForType:pasteboardType(mime)];
	if (data) return gio_onDataDrop(self.handle, (__bridge CFTypeRef)mime, (void *)data.bytes, data.length, point.x, self.bounds.size.height - point.y);
	if (![mime isEqualToString:@"text/uri-list"]) return NO;
	NSArray<NSURL *> *urls = [pboard readObjectsForClasses:@[[NSURL class]]
		options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
	NSMutableArray<NSString *> *paths = [NSMutableArray array];
	for (NSURL *url in urls) {
		if (url.isFileURL && url.path.length > 0) {
			[paths addObject:url.path];
		}
	}
	if (paths.count == 0 && [pboard.types containsObject:NSFilenamesPboardType]) {
		NSArray *names = [pboard propertyListForType:NSFilenamesPboardType];
		for (id name in names) {
			if ([name isKindOfClass:[NSString class]] && [name length] > 0) {
				[paths addObject:name];
			}
		}
	}
	if (paths.count == 0) {
		return NO;
	}
	return gio_onFileDrop(self.handle, (__bridge CFTypeRef)paths, paths.count,
		point.x, self.bounds.size.height - point.y);
}

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
	CALayer *layer = gio_layerFactory(self.presentWithTrans);
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
	self.dragEvent = nil;
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
	self.dragEvent = event;
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
	NSString *keys = [event charactersIgnoringModifiers];
	gio_onKeys(self.handle, (__bridge CFTypeRef)event, (__bridge CFTypeRef)keys, [event timestamp], [event modifierFlags], true);
}
- (void)flagsChanged:(NSEvent *)event {
	[self interpretKeyEvents:[NSArray arrayWithObject:event]];
	gio_onFlagsChanged(self.handle, [event modifierFlags]);
}
- (void)keyUp:(NSEvent *)event {
	NSString *keys = [event charactersIgnoringModifiers];
	gio_onKeys(self.handle, (__bridge CFTypeRef)event, (__bridge CFTypeRef)keys, [event timestamp], [event modifierFlags], false);
}
- (void)insertText:(id)string {
	gio_onText(self.handle, (__bridge CFTypeRef)string);
}
- (void)doCommandBySelector:(SEL)action {
	if (!gio_onCommandBySelector(self.handle)) {
		[super doCommandBySelector:action];
	}
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
	gio_onDraw(self.handle);
}
- (void)applicationDidHide:(NSNotification *)notification {
	gio_onDraw(self.handle);
}
- (void)dealloc {
	gio_onDestroy(self.handle);
}
- (BOOL) becomeFirstResponder {
	gio_onFocus(self.handle, 1);
	return [super becomeFirstResponder];
}
- (BOOL)acceptsFirstResponder {
    return YES;
}
- (BOOL) resignFirstResponder {
	gio_onFocus(self.handle, 0);
	return [super resignFirstResponder];
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
	if ([NSCursor respondsToSelector:cursorName]) {
		id object = [NSCursor performSelector:cursorName];
		if ([object isKindOfClass:[NSCursor class]]) {
			[(NSCursor*)object set];
			return;
		}
	}
	[fallback set];
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

CFTypeRef gio_createWindow(CFTypeRef viewRef, CGFloat width, CGFloat height) {
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
		NSView *view = (__bridge NSView *)viewRef;
		[window setContentView:view];
		window.delegate = globalWindowDel;
		return (__bridge_retained CFTypeRef)window;
	}
}

CFTypeRef gio_createView(int presentWithTrans) {
	@autoreleasepool {
		NSRect frame = NSMakeRect(0, 0, 0, 0);
		GioView* view = [[GioView alloc] initWithFrame:frame];
		view.presentWithTrans = presentWithTrans ? YES : NO;
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
}
- (void)application:(NSApplication *)application openURLs:(NSArray<NSURL *> *)urls {
	for (NSURL *url in urls) {
		gio_onOpenURI((__bridge CFTypeRef)url.absoluteString);
	}
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

@interface AppListener : NSObject
@end

static AppListener *appListener;

@implementation AppListener
- (void)launchFinished:(NSNotification *)notification {
	appListener = nil;
	gio_onFinishLaunching();
}
@end

void gio_init() {
	@autoreleasepool {
		appListener = [[AppListener alloc] init];
		[[NSNotificationCenter defaultCenter] addObserver:appListener
												 selector:@selector(launchFinished:)
													 name:NSApplicationDidFinishLaunchingNotification
												   object:nil];
	}
}
