package video

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// RawInput reads raw RGBA frames of the given size from stdin.
func RawInput(width, height, fps int) []string {
	return []string{
		"-f", "rawvideo", "-pix_fmt", "rgba",
		"-video_size", fmt.Sprintf("%dx%d", width, height),
		"-framerate", strconv.Itoa(fps),
		"-i", "pipe:0",
	}
}

// DDAGrabInput captures a Windows display with the Desktop Duplication API.
// output is the zero-based DXGI output index on the default adapter.
// The cursor is not drawn; the receiver overlays it.
func DDAGrabInput(output, fps int) []string {
	return []string{
		"-nostdin",
		"-f", "lavfi",
		"-i", fmt.Sprintf("ddagrab=output_idx=%d:framerate=%d:draw_mouse=0", output, fps),
	}
}

// AVFoundationInput captures a macOS screen. device is the AVFoundation video
// device index (see ParseAVFoundationScreens), not the screen number.
func AVFoundationInput(device, fps int) []string {
	return []string{
		"-nostdin",
		"-f", "avfoundation",
		"-capture_cursor", "0",
		"-framerate", strconv.Itoa(fps),
		"-i", fmt.Sprintf("%d:none", device),
	}
}

// PixelFormat returns the input pixel format the encoder expects.
func PixelFormat(encoder string) string {
	if strings.HasSuffix(encoder, "_qsv") {
		return "nv12"
	}
	return "yuv420p"
}

// SoftwareFilter crops frames in system memory to an even size and converts
// them to the encoder's pixel format.
func SoftwareFilter(width, height int, encoder string) string {
	w, h := EvenSize(width, height)
	return fmt.Sprintf("crop=%d:%d:0:0,format=%s", w, h, PixelFormat(encoder))
}

// DDAGrabFilter prepares ddagrab's GPU frames for the encoder. Intel Quick Sync
// encoders take the frames without leaving the GPU; every other encoder gets
// them downloaded to system memory first.
func DDAGrabFilter(width, height int, encoder string) string {
	if strings.HasSuffix(encoder, "_qsv") && width%2 == 0 && height%2 == 0 {
		return "hwmap=derive_device=qsv,format=qsv,vpp_qsv=format=nv12"
	}
	return "hwdownload,format=bgra," + SoftwareFilter(width, height, encoder)
}

// ScaleFilter resizes frames to exactly width x height (rounded down to even)
// and converts them to the encoder's pixel format.
func ScaleFilter(width, height int, encoder string) string {
	w, h := EvenSize(width, height)
	return fmt.Sprintf("scale=%d:%d,format=%s", w, h, PixelFormat(encoder))
}

var avfScreenRe = regexp.MustCompile(`\[(\d+)\] Capture screen (\d+)`)

// ParseAVFoundationScreens maps screen numbers to AVFoundation device indexes,
// from the output of `ffmpeg -f avfoundation -list_devices true -i ""`.
func ParseAVFoundationScreens(output string) map[int]int {
	screens := make(map[int]int)
	for _, m := range avfScreenRe.FindAllStringSubmatch(output, -1) {
		device, _ := strconv.Atoi(m[1])
		screen, _ := strconv.Atoi(m[2])
		screens[screen] = device
	}
	return screens
}
