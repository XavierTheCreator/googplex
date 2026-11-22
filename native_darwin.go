// Native macOS bits Wails v2 doesn't cover: the app's appearance at runtime,
// and hiding the pop-out's Dock icon.
package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// mode: 0 follows the system, 1 light, 2 dark.
static void gpSetAppearance(int mode) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSAppearance *a = nil;
		if (mode == 1) a = [NSAppearance appearanceNamed:NSAppearanceNameAqua];
		if (mode == 2) a = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
		[NSApp setAppearance:a];
	});
}

static void gpHideDockIcon(void) {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
	});
}
*/
import "C"

// hideDockIcon keeps the pop-out from adding a second Googleplex icon to the
// Dock. Wails v2 has no option for this, so it is set directly.
func hideDockIcon() { C.gpHideDockIcon() }

// setAppAppearance switches the whole app's native look (title bar, traffic
// lights, system controls) while it runs; Wails v2 only sets it at startup.
func setAppAppearance(theme string) {
	mode := 0
	switch theme {
	case "light":
		mode = 1
	case "dark":
		mode = 2
	}
	C.gpSetAppearance(C.int(mode))
}
