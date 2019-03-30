// SPDX-License-Identifier: Unlicense OR MIT

__attribute__ ((visibility ("hidden"))) void gio_showTextInput(CFTypeRef viewRef);
__attribute__ ((visibility ("hidden"))) void gio_hideTextInput(CFTypeRef viewRef);
__attribute__ ((visibility ("hidden"))) void gio_addLayerToView(CFTypeRef viewRef, CFTypeRef layerRef);
__attribute__ ((visibility ("hidden"))) void gio_updateView(CFTypeRef viewRef, CFTypeRef layerRef);
__attribute__ ((visibility ("hidden"))) void gio_removeLayer(CFTypeRef layerRef);
__attribute__ ((visibility ("hidden"))) void gio_setAnimating(CFTypeRef viewRef, int anim);
