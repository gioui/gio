// SPDX-License-Identifier: Unlicense OR MIT

@import Dispatch;

#include "os_darwin.h"
#include "_cgo_export.h"

void gio_wakeupMainThread(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		gio_dispatchMainFuncs();
	});
}
