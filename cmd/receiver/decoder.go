package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"genloya/screen-sharer/internal/proc"
	"genloya/screen-sharer/internal/protocol"
	"genloya/screen-sharer/internal/video"
)

// rawFrame is one decoded RGBA frame.
type rawFrame struct {
	width, height int
	pix           []byte
}

// framePool recycles frame buffers; frames can be tens of megabytes each.
var framePool sync.Pool

func getFrameBuf(size int) []byte {
	if b, ok := framePool.Get().([]byte); ok && cap(b) >= size {
		return b[:size]
	}
	return make([]byte, size)
}

func putFrameBuf(b []byte) {
	framePool.Put(b)
}

// decoder is an ffmpeg process that turns the encoded bitstream into RGBA frames.
type decoder struct {
	cmd   *exec.Cmd
	group *proc.Group
	stdin io.WriteCloser
	done  chan struct{}
}

// startDecoder launches ffmpeg for the stream described by hello and calls
// onFrame for every decoded frame. onFrame takes ownership of the frame buffer.
func startDecoder(ffmpeg string, hello protocol.Hello, onFrame func(rawFrame)) (*decoder, error) {
	codec, err := video.ParseCodec(hello.Codec)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(ffmpeg, video.DecoderArgs(codec)...)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg stdout: %w", err)
	}
	group, err := proc.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("start ffmpeg: %w", err)
	}

	d := &decoder{cmd: cmd, group: group, stdin: stdin, done: make(chan struct{})}
	go func() {
		defer close(d.done)
		size := hello.Width * hello.Height * 4
		for {
			pix := getFrameBuf(size)
			if _, err := io.ReadFull(stdout, pix); err != nil {
				return
			}
			onFrame(rawFrame{width: hello.Width, height: hello.Height, pix: pix})
		}
	}()
	return d, nil
}

// Write feeds a chunk of the encoded bitstream to ffmpeg.
func (d *decoder) Write(p []byte) (int, error) {
	return d.stdin.Write(p)
}

// Close stops ffmpeg and waits for it to exit. Pending frames are discarded,
// so ffmpeg is killed before its input is closed to skip a pointless flush.
func (d *decoder) Close() {
	d.group.Kill()
	d.stdin.Close()
	<-d.done
	d.cmd.Wait()
}
