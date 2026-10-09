// Command sender connects to a receiver and streams a display as H.264/H.265 video.
//
// ffmpeg captures and encodes the display (ddagrab on Windows, avfoundation on
// macOS, or frames piped from Go as a fallback), and the encoded bitstream is
// forwarded to the receiver. When the receiver closes the connection (for
// example because another sender took over), the sender exits.
package main

import (
	"context"
	"flag"
	"image"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"time"

	"genloya/screen-sharer/internal/proc"
	"genloya/screen-sharer/internal/protocol"
	"genloya/screen-sharer/internal/video"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:9000", "receiver address (host:port)")
	display := flag.Int("display", 1, "display to stream (1-based, see -list-displays)")
	fps := flag.Int("fps", 30, "frames per second")
	codecName := flag.String("codec", "h264", "video codec: h264 or h265")
	encoder := flag.String("encoder", "", "ffmpeg encoder (default libx264/libx265; e.g. h264_qsv, h264_nvenc, h264_videotoolbox)")
	bitrate := flag.String("bitrate", "4M", "target video bitrate")
	capture := flag.String("capture", "", "capture method: "+strings.Join(captureMethods, " or ")+" (default "+captureMethods[0]+")")
	list := flag.Bool("list-displays", false, "list displays for the capture method and exit")
	ffmpegName := flag.String("ffmpeg", "ffmpeg", "ffmpeg executable name or path")
	flag.Parse()

	ffmpeg, err := video.FindFFmpeg(*ffmpegName)
	if err != nil {
		log.Fatal(err)
	}
	if *list {
		if err := listDisplays(*capture); err != nil {
			log.Fatal(err)
		}
		return
	}
	codec, err := video.ParseCodec(*codecName)
	if err != nil {
		log.Fatal(err)
	}
	if *encoder == "" {
		*encoder = codec.DefaultEncoder()
	}
	if *fps <= 0 {
		log.Fatal("fps must be greater than 0")
	}

	src, err := openSource(ffmpeg, *capture, *display-1, *fps, *encoder)
	if err != nil {
		log.Fatal(err)
	}

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	conn, err := dial(sigCtx, *addr)
	if err != nil {
		if sigCtx.Err() != nil {
			return
		}
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(sigCtx)
	defer cancel()
	go func() {
		<-ctx.Done()
		conn.Close()
	}()

	// The receiver never sends data; a read returning means it hung up.
	go func() {
		io.Copy(io.Discard, conn)
		cancel()
	}()

	out := &msgWriter{w: conn}
	w, h := video.EvenSize(src.size.X, src.size.Y)
	hello, err := protocol.EncodeHello(protocol.Hello{Codec: string(codec), Width: w, Height: h})
	if err != nil {
		log.Fatalf("hello: %v", err)
	}
	if err := out.write(protocol.MsgHello, hello); err != nil {
		log.Fatalf("hello: %v", err)
	}

	cmd := exec.Command(ffmpeg, video.EncoderArgs(video.EncoderConfig{
		Input:   src.input,
		Filter:  src.filter,
		Codec:   codec,
		Encoder: *encoder,
		FPS:     *fps,
		Bitrate: *bitrate,
	})...)
	cmd.Stderr = os.Stderr
	var stdin io.WriteCloser
	if src.pipe {
		if stdin, err = cmd.StdinPipe(); err != nil {
			log.Fatalf("ffmpeg stdin: %v", err)
		}
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("ffmpeg stdout: %v", err)
	}
	group, err := proc.Start(cmd)
	if err != nil {
		log.Fatalf("start ffmpeg: %v", err)
	}
	go func() {
		<-ctx.Done()
		group.Kill()
	}()
	log.Printf("connected to receiver %s, streaming display %d (%dx%d, %s, %s via %s)",
		*addr, *display, w, h, src.desc, codec, *encoder)

	forwarded := make(chan struct{})
	go func() {
		defer close(forwarded)
		forward(stdout, out)
		cancel()
	}()
	go trackCursor(ctx, out, src.bounds, src.size)

	if src.pipe {
		capturePipe(ctx, stdin, src.display, src.size, *fps)
		cancel()
		stdin.Close()
	} else {
		<-ctx.Done()
	}
	<-forwarded
	cmd.Wait()
	log.Print("stream ended")
}

// dial connects to the receiver, retrying with backoff until it is reachable.
func dial(ctx context.Context, addr string) (net.Conn, error) {
	var d net.Dialer
	backoff := time.Second
	for {
		dctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		conn, err := d.DialContext(dctx, "tcp", addr)
		cancel()
		if err == nil {
			return conn, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		log.Printf("dial %s: %v (retrying in %s)", addr, err, backoff)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 10*time.Second)
	}
}

// msgWriter serializes messages written from several goroutines.
type msgWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (m *msgWriter) write(t protocol.MsgType, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return protocol.WriteMessage(m.w, t, payload)
}

// trackCursor sends the pointer position, mapped to frame pixels, whenever it changes.
// bounds is the display area in the cursor's coordinate space; frame is the captured size.
func trackCursor(ctx context.Context, out *msgWriter, bounds image.Rectangle, frame image.Point) {
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()

	var last protocol.Cursor
	first := true
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		var c protocol.Cursor
		if p, ok := cursorPosition(); ok && p.In(bounds) {
			c = protocol.Cursor{
				X:       int32((p.X - bounds.Min.X) * frame.X / bounds.Dx()),
				Y:       int32((p.Y - bounds.Min.Y) * frame.Y / bounds.Dy()),
				Visible: true,
			}
		}
		if !first && c == last {
			continue
		}
		first, last = false, c
		if err := out.write(protocol.MsgCursor, protocol.EncodeCursor(c)); err != nil {
			return
		}
	}
}

// forward sends the encoded bitstream to the receiver as it is produced.
func forward(encoded io.Reader, out *msgWriter) {
	buf := make([]byte, 64<<10)
	for {
		n, err := encoded.Read(buf)
		if n > 0 {
			if werr := out.write(protocol.MsgVideo, buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			if err != io.EOF {
				log.Printf("ffmpeg output: %v", err)
			}
			return
		}
	}
}
