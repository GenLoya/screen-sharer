//go:build windows

package main

import (
	"fmt"
	"syscall"

	"genloya/screen-sharer/internal/video"
)

var captureMethods = []string{"ddagrab", "pipe"}

func init() {
	// Work in physical pixels everywhere (DXGI bounds, GetCursorPos, screenshots),
	// otherwise scaled displays report virtualized coordinates.
	proc := syscall.NewLazyDLL("user32.dll").NewProc("SetProcessDpiAwarenessContext")
	if proc.Find() == nil {
		const perMonitorAwareV2 = ^uintptr(3) // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 (-4)
		proc.Call(perMonitorAwareV2)
	}
}

func openSource(_ string, method string, display, fps int, encoder string) (*source, error) {
	switch method {
	case "", "ddagrab":
		outputs, err := dxgiOutputs()
		if err != nil {
			return nil, err
		}
		if display < 0 || display >= len(outputs) {
			return nil, fmt.Errorf("invalid display %d (found %d displays on the primary GPU)", display+1, len(outputs))
		}
		o := outputs[display]
		size := o.bounds.Size()
		return &source{
			desc:   fmt.Sprintf("ddagrab %s", o.name),
			input:  video.DDAGrabInput(display, fps),
			filter: video.DDAGrabFilter(size.X, size.Y, encoder),
			size:   size,
			bounds: o.bounds,
		}, nil
	case "pipe":
		return pipeSource(display, fps, encoder)
	}
	return nil, fmt.Errorf("unknown capture method %q (use ddagrab or pipe)", method)
}

func listDisplays(method string) error {
	if method == "pipe" {
		listScreenshotDisplays()
		return nil
	}
	outputs, err := dxgiOutputs()
	if err != nil {
		return err
	}
	for i, o := range outputs {
		fmt.Printf("%d: %s %dx%d at (%d,%d)\n", i+1, o.name, o.bounds.Dx(), o.bounds.Dy(), o.bounds.Min.X, o.bounds.Min.Y)
	}
	return nil
}
