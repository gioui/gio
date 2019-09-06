// SPDX-License-Identifier: Unlicense OR MIT

// +build darwin,ios

@import UIKit;

#include <stdint.h>
#include "_cgo_export.h"
#include "os_ios.h"
#include "framework_ios.h"

@interface GioView: UIView <UIKeyInput>
- (void)setAnimating:(BOOL)anim;
@end

@interface GioViewController : UIViewController
@property(weak) UIScreen *screen;
@end

static void redraw(CFTypeRef viewRef, BOOL sync) {
	UIView *v = (__bridge UIView *)viewRef;
	CGFloat scale = v.layer.contentsScale;
	// Use 163 as the standard ppi on iOS.
	CGFloat dpi = 163*scale;
	CGFloat sdpi = dpi;
	UIEdgeInsets insets = v.layoutMargins;
	if (@available(iOS 11.0, tvOS 11.0, *)) {
		UIFontMetrics *metrics = [UIFontMetrics defaultMetrics];
		sdpi = [metrics scaledValueForValue:sdpi];
		insets = v.safeAreaInsets;
	}
	onDraw(viewRef, dpi, sdpi, v.bounds.size.width*scale, v.bounds.size.height*scale, sync,
			insets.top*scale, insets.right*scale, insets.bottom*scale, insets.left*scale);
}

@implementation GioAppDelegate
- (BOOL)application:(UIApplication *)application didFinishLaunchingWithOptions:(NSDictionary *)launchOptions {
	gio_runMain();

	self.window = [[UIWindow alloc] initWithFrame:[[UIScreen mainScreen] bounds]];
	GioViewController *controller = [[GioViewController alloc] initWithNibName:nil bundle:nil];
	controller.screen = self.window.screen;
    self.window.rootViewController = controller;
    [self.window makeKeyAndVisible];
	return YES;
}

- (void)applicationWillResignActive:(UIApplication *)application {
}

- (void)applicationDidEnterBackground:(UIApplication *)application {
	GioViewController *vc = (GioViewController *)self.window.rootViewController;
	UIView *drawView = vc.view.subviews[0];
	if (drawView != nil) {
		onStop((__bridge CFTypeRef)drawView);
	}
}

- (void)applicationWillEnterForeground:(UIApplication *)application {
	GioViewController *c = (GioViewController*)self.window.rootViewController;
	UIView *drawView = c.view.subviews[0];
	if (drawView != nil) {
		CFTypeRef viewRef = (__bridge CFTypeRef)drawView;
		redraw(viewRef, YES);
	}
}
@end

@implementation GioViewController
CGFloat _keyboardHeight;

- (void)loadView {
	CGRect zeroFrame = CGRectMake(0, 0, 0, 0);
	self.view = [[UIView alloc] initWithFrame:zeroFrame];
	self.view.layoutMargins = UIEdgeInsetsMake(0, 0, 0, 0);
	UIView *drawView = [[GioView alloc] initWithFrame:zeroFrame];
	[self.view addSubview: drawView];
#ifndef TARGET_OS_TV
	drawView.multipleTouchEnabled = YES;
#endif
	drawView.contentScaleFactor = self.screen.nativeScale;
	drawView.preservesSuperviewLayoutMargins = YES;
	drawView.layoutMargins = UIEdgeInsetsMake(0, 0, 0, 0);
	onCreate((__bridge CFTypeRef)drawView);
	[[NSNotificationCenter defaultCenter] addObserver:self
											 selector:@selector(keyboardWillChange:)
												 name:UIKeyboardWillShowNotification
											   object:nil];
	[[NSNotificationCenter defaultCenter] addObserver:self
											 selector:@selector(keyboardWillChange:)
												 name:UIKeyboardWillChangeFrameNotification
											   object:nil];
	[[NSNotificationCenter defaultCenter] addObserver:self
											 selector:@selector(keyboardWillHide:)
												 name:UIKeyboardWillHideNotification
											   object:nil];
}

- (void)viewDidDisappear:(BOOL)animated {
	[super viewDidDisappear:animated];
	CFTypeRef viewRef = (__bridge CFTypeRef)self.view.subviews[0];
	onDestroy(viewRef);
}

- (void)viewDidLayoutSubviews {
	[super viewDidLayoutSubviews];
	UIView *view = self.view.subviews[0];
	CGRect frame = self.view.bounds;
	// Adjust view bounds to make room for the keyboard.
	frame.size.height -= _keyboardHeight;
	view.frame = frame;
	redraw((__bridge CFTypeRef)view, YES);
}

- (void)didReceiveMemoryWarning {
	onLowMemory();
	[super didReceiveMemoryWarning];
}

- (void)keyboardWillChange:(NSNotification *)note {
	NSDictionary *userInfo = note.userInfo;
	CGRect f = [userInfo[UIKeyboardFrameEndUserInfoKey] CGRectValue];
	_keyboardHeight = f.size.height;
	[self.view setNeedsLayout];
}

- (void)keyboardWillHide:(NSNotification *)note {
	_keyboardHeight = 0.0;
	[self.view setNeedsLayout];
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

- (void)willMoveToWindow:(UIWindow *)newWindow {
	if (self.window != nil) {
		[[NSNotificationCenter defaultCenter] removeObserver:self
														name:UIWindowDidBecomeKeyNotification
													  object:self.window];
		[[NSNotificationCenter defaultCenter] removeObserver:self
														name:UIWindowDidResignKeyNotification
													  object:self.window];
	}
	[[NSNotificationCenter defaultCenter] addObserver:self
											 selector:@selector(onWindowDidBecomeKey:)
												 name:UIWindowDidBecomeKeyNotification
											   object:newWindow];
	[[NSNotificationCenter defaultCenter] addObserver:self
											 selector:@selector(onWindowDidResignKey:)
												 name:UIWindowDidResignKeyNotification
											   object:newWindow];
}

- (void)onWindowDidBecomeKey:(NSNotification *)note {
	if (self.isFirstResponder) {
		onFocus((__bridge CFTypeRef)self, YES);
	}
}

- (void)onWindowDidResignKey:(NSNotification *)note {
	if (self.isFirstResponder) {
		onFocus((__bridge CFTypeRef)self, NO);
	}
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
