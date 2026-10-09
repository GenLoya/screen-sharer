package main

import (
	"context"
	"fmt"
	"image"
	"io"
	"log"
	"time"

	"github.com/kbinani/screenshot"

	"genloya/screen-sharer/internal/video"
)

// source describes how ffmpeg gets the frames of the shared display.
type source struct {
	desc    string          // human-readable description for logs
	input   []string        // ffmpeg input arguments
	filter  string          // -vf chain producing encoder-ready frames
	size    image.Point     // captured frame size in pixels
	bounds  image.Rectangle // display area in cursor coordinates
	pipe    bool            // frames are captured in Go and written to ffmpeg's stdin
	display int             // zero-based display index for pipe capture
}

// pipeSource captures the display in Go and pipes raw frames into ffmpeg.
// It works everywhere but costs more CPU than a native ffmpeg capture device.
func pipeSource(display, fps int, encoder string) (*source, error) {
	if n := screenshot.NumActiveDisplays(); display < 0 || display >= n {
		return nil, fmt.Errorf("invalid display %d (found %d active displays)", display+1, n)
	}
	// Capture once to learn the real pixel size (it differs from the bounds on HiDPI screens).
	img, err := screenshot.CaptureDisplay(display)
	if err != nil {
		return nil, fmt.Errorf("capture: %w", err)
	}
	size := img.Bounds().Size()
	return &source{
		desc:    "screenshot pipe",
		input:   video.RawInput(size.X, size.Y, fps),
		filter:  video.SoftwareFilter(size.X, size.Y, encoder),
		size:    size,
		bounds:  screenshot.GetDisplayBounds(display),
		pipe:    true,
		display: display,
	}, nil
}

// listScreenshotDisplays prints the displays as seen by the screenshot library.
func listScreenshotDisplays() {
	for i := range screenshot.NumActiveDisplays() {
		b := screenshot.GetDisplayBounds(i)
		fmt.Printf("%d: %dx%d at (%d,%d)\n", i+1, b.Dx(), b.Dy(), b.Min.X, b.Min.Y)
	}
}

// capturePipe grabs the display at a fixed rate and writes raw RGBA frames to the encoder.
func capturePipe(ctx context.Context, encoder io.Writer, display int, size image.Point, fps int) {
	ticker := time.NewTicker(time.Second / time.Duration(fps))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		img, err := screenshot.CaptureDisplay(display)
		if err != nil {
			log.Printf("capture: %v", err)
			continue
		}
		if img.Bounds().Size() != size {
			log.Printf("display resolution changed to %v; restart the sender", img.Bounds().Size())
			return
		}
		if err := writeRGBA(encoder, img); err != nil {
			return
		}
	}
}

// writeRGBA writes the image's pixels without row padding.
func writeRGBA(w io.Writer, img *image.RGBA) error {
	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	rowLen := width * 4
	if img.Stride == rowLen {
		_, err := w.Write(img.Pix[:rowLen*height])
		return err
	}
	for y := 0; y < height; y++ {
		row := img.Pix[y*img.Stride : y*img.Stride+rowLen]
		if _, err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}
