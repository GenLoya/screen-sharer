//go:build darwin

package main

import (
	"fmt"
	"os/exec"

	"github.com/kbinani/screenshot"

	"genloya/screen-sharer/internal/video"
)

var captureMethods = []string{"avfoundation", "pipe"}

func openSource(ffmpeg, method string, display, fps int, encoder string) (*source, error) {
	switch method {
	case "", "avfoundation":
		// Reuse the pipe source to validate the display and learn its pixel size and bounds.
		// Both screenshot and AVFoundation number screens in CGGetActiveDisplayList order.
		src, err := pipeSource(display, fps, encoder)
		if err != nil {
			return nil, err
		}
		device, err := avfoundationDevice(ffmpeg, display)
		if err != nil {
			return nil, err
		}
		src.desc = fmt.Sprintf("avfoundation screen %d (device %d)", display, device)
		src.input = video.AVFoundationInput(device, fps)
		// AVFoundation may deliver frames at a different scale; pin them to the announced size.
		src.filter = video.ScaleFilter(src.size.X, src.size.Y, encoder)
		src.pipe = false
		return src, nil
	case "pipe":
		return pipeSource(display, fps, encoder)
	}
	return nil, fmt.Errorf("unknown capture method %q (use avfoundation or pipe)", method)
}

// avfoundationDevice finds the AVFoundation device index of a screen.
// Cameras come first in the device list, so the index differs from the screen number.
func avfoundationDevice(ffmpeg string, screen int) (int, error) {
	// ffmpeg exits with an error after listing devices; only the output matters.
	out, _ := exec.Command(ffmpeg, "-hide_banner", "-f", "avfoundation", "-list_devices", "true", "-i", "").CombinedOutput()
	device, ok := video.ParseAVFoundationScreens(string(out))[screen]
	if !ok {
		return 0, fmt.Errorf("avfoundation has no \"Capture screen %d\" device; "+
			"grant Screen Recording permission to your terminal or use -capture pipe", screen)
	}
	return device, nil
}

func listDisplays(string) error {
	if screenshot.NumActiveDisplays() == 0 {
		return fmt.Errorf("no active displays found")
	}
	listScreenshotDisplays()
	return nil
}
