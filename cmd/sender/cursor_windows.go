//go:build windows

package main

import (
	"image"
	"syscall"
	"unsafe"
)

var procGetCursorPos = syscall.NewLazyDLL("user32.dll").NewProc("GetCursorPos")

// cursorPosition returns the pointer position in global screen coordinates.
func cursorPosition() (image.Point, bool) {
	var pt struct{ X, Y int32 }
	if r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt))); r == 0 {
		return image.Point{}, false
	}
	return image.Pt(int(pt.X), int(pt.Y)), true
}
