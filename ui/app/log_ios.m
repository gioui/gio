// SPDX-License-Identifier: Unlicense OR MIT

// +build ios

@import Foundation;

#include "log_ios.h"

void nslog(char *str) {
	NSLog(@"%@", @(str));
}
