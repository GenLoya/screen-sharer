// Package video builds the ffmpeg command lines used to encode and decode the stream.
//
// ffmpeg must be installed on both the sender (encoding) and the receiver (decoding).
package video

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Codec is the video codec of the stream, named after its ffmpeg raw format.
type Codec string

const (
	H264 Codec = "h264"
	HEVC Codec = "hevc"
)

// ParseCodec accepts user-facing codec names such as "h264" or "h265".
func ParseCodec(s string) (Codec, error) {
	switch strings.ToLower(s) {
	case "h264", "avc":
		return H264, nil
	case "h265", "hevc":
		return HEVC, nil
	}
	return "", fmt.Errorf("unsupported codec %q (use h264 or h265)", s)
}

// DefaultEncoder returns the software encoder shipped with common ffmpeg builds.
func (c Codec) DefaultEncoder() string {
	if c == HEVC {
		return "libx265"
	}
	return "libx264"
}

// FindFFmpeg resolves the ffmpeg executable, explaining how to install it if missing.
func FindFFmpeg(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("ffmpeg is required but was not found (%v)\n"+
			"install it and make sure it is on your PATH:\n"+
			"  Windows: winget install Gyan.FFmpeg\n"+
			"  macOS:   brew install ffmpeg", err)
	}
	return path, nil
}

// EvenSize rounds dimensions down to even numbers, as required by yuv420p.
func EvenSize(width, height int) (int, int) {
	return width &^ 1, height &^ 1
}

// EncoderConfig describes how the sender captures and encodes the display.
type EncoderConfig struct {
	Input   []string // ffmpeg input arguments, e.g. from RawInput or DDAGrabInput
	Filter  string   // -vf chain that turns the input into encoder-ready frames
	Codec   Codec
	Encoder string // ffmpeg encoder name, e.g. libx264 or h264_qsv
	FPS     int
	Bitrate string // ffmpeg bitrate, e.g. 4M
}

// EncoderArgs captures from the configured input and writes a raw bitstream to stdout.
func EncoderArgs(c EncoderConfig) []string {
	args := []string{"-hide_banner", "-loglevel", "error"}
	args = append(args, c.Input...)
	args = append(args,
		"-vf", c.Filter,
		"-c:v", c.Encoder,
		"-b:v", c.Bitrate,
		"-g", strconv.Itoa(c.FPS*2),
		"-bf", "0",
	)
	switch c.Encoder {
	case "libx264":
		args = append(args, "-preset", "ultrafast", "-tune", "zerolatency")
	case "libx265":
		args = append(args, "-preset", "ultrafast", "-tune", "zerolatency", "-x265-params", "log-level=error")
	case "h264_videotoolbox", "hevc_videotoolbox":
		args = append(args, "-realtime", "1")
	}
	return append(args, "-flush_packets", "1", "-f", string(c.Codec), "pipe:1")
}

// DecoderArgs reads a raw bitstream from stdin and writes raw RGBA frames to stdout.
func DecoderArgs(c Codec) []string {
	return []string{
		"-hide_banner", "-loglevel", "error",
		"-flags", "low_delay",
		"-probesize", "32", "-analyzeduration", "0",
		"-f", string(c), "-i", "pipe:0",
		"-f", "rawvideo", "-pix_fmt", "rgba",
		"-flush_packets", "1", "pipe:1",
	}
}
