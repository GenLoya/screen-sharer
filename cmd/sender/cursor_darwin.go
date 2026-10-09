//go:build darwin

package main

import (
	"image"
	"log"
	"sync"

	"github.com/ebitengine/purego"
)

type cgPoint struct{ X, Y float64 }

var (
	cursorAPIOnce sync.Once
	cursorAPIErr  error

	cgEventCreate      func(source uintptr) uintptr
	cgEventGetLocation func(event uintptr) cgPoint
	cfRelease          func(ref uintptr)
)

func loadCursorAPI() error {
	cg, err := purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return err
	}
	cf, err := purego.Dlopen("/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return err
	}
	purego.RegisterLibFunc(&cgEventCreate, cg, "CGEventCreate")
	purego.RegisterLibFunc(&cgEventGetLocation, cg, "CGEventGetLocation")
	purego.RegisterLibFunc(&cfRelease, cf, "CFRelease")
	return nil
}

// cursorPosition returns the pointer position in global display coordinates
// (points, origin at the top-left of the main display), the same space as
// screenshot.GetDisplayBounds.
func cursorPosition() (image.Point, bool) {
	cursorAPIOnce.Do(func() {
		if cursorAPIErr = loadCursorAPI(); cursorAPIErr != nil {
			log.Printf("cursor tracking disabled: %v", cursorAPIErr)
		}
	})
	if cursorAPIErr != nil {
		return image.Point{}, false
	}

	event := cgEventCreate(0)
	if event == 0 {
		return image.Point{}, false
	}
	defer cfRelease(event)
	p := cgEventGetLocation(event)
	return image.Pt(int(p.X), int(p.Y)), true
}
