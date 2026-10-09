//go:build !windows && !darwin

package main

import "image"

// cursorPosition is not implemented on this platform; the cursor is not streamed.
func cursorPosition() (image.Point, bool) {
	return image.Point{}, false
}
