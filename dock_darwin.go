//go:build darwin && cgo
// +build darwin,cgo

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>
	void HideDockIcon(int shouldHide) {
		if (shouldHide)
	    	[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
		else
			[NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
	    return;
	}
*/
import "C"

func hideFromDock(shouldHide bool) {
	if shouldHide {
		C.HideDockIcon(1)
	} else {
		C.HideDockIcon(0)
	}
}
