package main

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

// cursorShape is a classic arrow pointer: X is the outline, o the fill.
// The hotspot is the top-left pixel.
var cursorShape = []string{
	"X",
	"XX",
	"XoX",
	"XooX",
	"XoooX",
	"XooooX",
	"XoooooX",
	"XooooooX",
	"XoooooooX",
	"XooooooooX",
	"XoooooooooX",
	"XooooooXXXXX",
	"XoooXooX",
	"XooXXooX",
	"XoX XooX",
	"XX   XooX",
	"X    XooX",
	"      XooX",
	"      XooX",
	"       XX",
}

func newCursorImage() *ebiten.Image {
	width := 0
	for _, row := range cursorShape {
		width = max(width, len(row))
	}
	img := image.NewRGBA(image.Rect(0, 0, width, len(cursorShape)))
	for y, row := range cursorShape {
		for x, ch := range row {
			switch ch {
			case 'X':
				img.Set(x, y, color.Black)
			case 'o':
				img.Set(x, y, color.White)
			}
		}
	}
	return ebiten.NewImageFromImage(img)
}
