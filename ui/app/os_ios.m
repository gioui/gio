// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,ios

@import UIKit;

#include <stdint.h>
#include "_cgo_export.h"
#include "os_ios.h"
#include "framework_ios.h"

@interface GioViewController : UIViewController
@property UIScreen *screen;
@end

@interface GioView: UIView <UIKeyInput>
- (void)setAnimating:(BOOL)anim;
@end

static void redraw(CFTypeRef viewRef, BOOL sync) {
	UIView *v = (__bridge UIView *)viewRef;
	CGFloat scale = v.layer.contentsScale;
	// Use 163 as the standard ppi on iOS.
	CGFloat dpi = 163*scale;
	CGFloat sdpi = dpi;
	if (@available(iOS 11.0, tvOS 11.0, *)) {
		UIFontMetrics *metrics = [UIFontMetrics defaultMetrics];
		sdpi = [metrics scaledValueForValue:sdpi];
	}
	onDraw(viewRef, dpi, sdpi, v.bounds.size.width*scale, v.bounds.size.height*scale, sync);
}

@implementation GioAppDelegate
- (BOOL)application:(UIApplication *)application didFinishLaunchingWithOptions:(NSDictionary *)launchOptions {
	self.window = [[UIWindow alloc] initWithFrame:[[UIScreen mainScreen] bounds]];
	GioViewController *controller = [[GioViewController alloc] initWithNibName:nil bundle:nil];
	controller.screen = self.window.screen;
    self.window.rootViewController = controller;
	[[NSNotificationCenter defaultCenter] addObserverForName:UIWindowDidBecomeKeyNotification object:nil queue:[NSOperationQueue mainQueue] usingBlock:^(NSNotification *notification) {
		UIView *view = self.window.rootViewController.view;
		if (view != nil)
			onFocus((__bridge CFTypeRef)view, YES);
	}];
	[[NSNotificationCenter defaultCenter] addObserverForName:UIWindowDidResignKeyNotification object:nil queue:[NSOperationQueue mainQueue] usingBlock:^(NSNotification *notification) {
		UIView *view = self.window.rootViewController.view;
		if (view != nil)
			onFocus((__bridge CFTypeRef)view, NO);
	}];
    [self.window makeKeyAndVisible];
	return YES;
}

- (void)applicationWillResignActive:(UIApplication *)application {
}

- (void)applicationDidEnterBackground:(UIApplication *)application {
	if (self.window.rootViewController.view != nil) {
		onStop((__bridge CFTypeRef)self.window.rootViewController.view);
	}
}

- (void)applicationWillEnterForeground:(UIApplication *)application {
	GioViewController *c = (GioViewController*)self.window.rootViewController;
	if (c.view != nil) {
		CFTypeRef viewRef = (__bridge CFTypeRef)c.view;
		redraw(viewRef, YES);
	}
}

- (void)applicationDidBecomeActive:(UIApplication *)application {
}

- (void)applicationWillTerminate:(UIApplication *)application {
}

- (void)applicationDidReceiveMemoryWarning:(UIApplication *)application {
	onLowMemory();
}
@end

@implementation GioViewController
- (void)loadView {
	CGRect zeroFrame = CGRectMake(0, 0, 0, 0);
	self.view = [[GioView alloc] initWithFrame:zeroFrame];
#ifndef TARGET_OS_TV
	self.view.multipleTouchEnabled = YES;
#endif
	self.view.contentScaleFactor = self.screen.nativeScale;
	onCreate((__bridge CFTypeRef)self.view);
}

- (void)viewWillAppear:(BOOL)animated {
	[super viewWillAppear:animated];
	CFTypeRef viewRef = (__bridge CFTypeRef)self.view;
	redraw(viewRef, YES);
}

- (void)viewDidDisappear:(BOOL)animated {
	[super viewDidDisappear:animated];
	CFTypeRef viewRef = (__bridge CFTypeRef)self.view;
	onDestroy(viewRef);
}

- (void)viewDidLayoutSubviews {
	redraw((__bridge CFTypeRef)self.view, YES);
}
@end

static void handleTouches(int last, UIView *view, NSSet<UITouch *> *touches, UIEvent *event) {
	CGFloat scale = view.contentScaleFactor;
	NSUInteger i = 0;
	NSUInteger n = [touches count];
	CFTypeRef viewRef = (__bridge CFTypeRef)view;
	for (UITouch *touch in touches) {
		CFTypeRef touchRef = (__bridge CFTypeRef)touch;
		i++;
		NSArray<UITouch *> *coalescedTouches = [event coalescedTouchesForTouch:touch];
		NSUInteger j = 0;
		NSUInteger m = [coalescedTouches count];
		for (UITouch *coalescedTouch in [event coalescedTouchesForTouch:touch]) {
			CGPoint loc = [coalescedTouch locationInView:view];
			j++;
			int lastTouch = last && i == n && j == m;
			onTouch(lastTouch, viewRef, touchRef, touch.phase, loc.x*scale, loc.y*scale, [coalescedTouch timestamp]);
		}
	}
}

@implementation GioView
CADisplayLink *displayLink;
NSArray<UIKeyCommand *> *_keyCommands;

- (void)onFrameCallback:(CADisplayLink *)link {
	redraw((__bridge CFTypeRef)self, NO);
}

- (instancetype)initWithFrame:(CGRect)frame {
	self = [super initWithFrame:frame];
	if (self) {
		__weak id weakSelf = self;
		displayLink = [CADisplayLink displayLinkWithTarget:weakSelf selector:@selector(onFrameCallback:)];
	}
	return self;
}

- (void)dealloc {
	[displayLink invalidate];
}

- (void)setAnimating:(BOOL)anim {
	NSRunLoop *runLoop = [NSRunLoop currentRunLoop];
	if (anim) {
		[displayLink addToRunLoop:runLoop forMode:[runLoop currentMode]];
	} else {
		[displayLink removeFromRunLoop:runLoop forMode:[runLoop currentMode]];
	}
}

- (void)touchesBegan:(NSSet<UITouch *> *)touches withEvent:(UIEvent *)event {
	handleTouches(0, self, touches, event);
}

- (void)touchesMoved:(NSSet<UITouch *> *)touches withEvent:(UIEvent *)event {
	handleTouches(0, self, touches, event);
}

- (void)touchesEnded:(NSSet<UITouch *> *)touches withEvent:(UIEvent *)event {
	handleTouches(1, self, touches, event);
}

- (void)touchesCancelled:(NSSet<UITouch *> *)touches withEvent:(UIEvent *)event {
	handleTouches(1, self, touches, event);
}

- (void)insertText:(NSString *)text {
	onText((__bridge CFTypeRef)self, (char *)text.UTF8String);
}

- (BOOL)canBecomeFirstResponder {
	return YES;
}

- (BOOL)hasText {
	return YES;
}

- (void)deleteBackward {
	onDeleteBackward((__bridge CFTypeRef)self);
}

- (void)onUpArrow {
	onUpArrow((__bridge CFTypeRef)self);
}

- (void)onDownArrow {
	onDownArrow((__bridge CFTypeRef)self);
}

- (void)onLeftArrow {
	onLeftArrow((__bridge CFTypeRef)self);
}

- (void)onRightArrow {
	onRightArrow((__bridge CFTypeRef)self);
}

- (NSArray<UIKeyCommand *> *)keyCommands {
	if (_keyCommands == nil) {
		_keyCommands = @[
			[UIKeyCommand keyCommandWithInput:UIKeyInputUpArrow
								modifierFlags:0
									   action:@selector(onUpArrow)],
			[UIKeyCommand keyCommandWithInput:UIKeyInputDownArrow
								modifierFlags:0
									   action:@selector(onDownArrow)],
			[UIKeyCommand keyCommandWithInput:UIKeyInputLeftArrow
								modifierFlags:0
									   action:@selector(onLeftArrow)],
			[UIKeyCommand keyCommandWithInput:UIKeyInputRightArrow
								modifierFlags:0
									   action:@selector(onRightArrow)]
		];
	}
	return _keyCommands;
}
@end

void gio_setAnimating(CFTypeRef viewRef, int anim) {
	GioView *view = (__bridge GioView *)viewRef;
	dispatch_async(dispatch_get_main_queue(), ^{
			[view setAnimating:(anim ? YES : NO)];
	});
}

void gio_showTextInput(CFTypeRef viewRef) {
	UIView *view = (__bridge UIView *)viewRef;
	dispatch_async(dispatch_get_main_queue(), ^{
		[view becomeFirstResponder];
	});
}

void gio_hideTextInput(CFTypeRef viewRef) {
	UIView *view = (__bridge UIView *)viewRef;
	dispatch_async(dispatch_get_main_queue(), ^{
		[view resignFirstResponder];
	});
}

void gio_addLayerToView(CFTypeRef viewRef, CFTypeRef layerRef) {
	UIView *view = (__bridge UIView *)viewRef;
	CALayer *layer = (__bridge CALayer *)layerRef;
	[view.layer addSublayer:layer];
}

void gio_updateView(CFTypeRef viewRef, CFTypeRef layerRef) {
	UIView *view = (__bridge UIView *)viewRef;
	CAEAGLLayer *layer = (__bridge CAEAGLLayer *)layerRef;
	layer.contentsScale = view.contentScaleFactor;
	layer.bounds = view.bounds;
}

void gio_removeLayer(CFTypeRef layerRef) {
	CALayer *layer = (__bridge CALayer *)layerRef;
	[layer removeFromSuperlayer];
}
