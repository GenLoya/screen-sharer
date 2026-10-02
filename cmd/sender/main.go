// Command sender connects to a receiver and streams a display as MJPEG over TCP.
//
// When the receiver closes the connection (for example because another sender
// took over), the sender exits.
package main

import (
	"bufio"
	"bytes"
	"context"
	"flag"
	"image/jpeg"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/kbinani/screenshot"

	"genloya/screen-sharer/internal/protocol"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:9000", "receiver address (host:port)")
	display := flag.Int("display", 1, "display to stream (1-based)")
	fps := flag.Int("fps", 15, "frames per second")
	quality := flag.Int("quality", 70, "JPEG quality (1-100)")
	flag.Parse()

	n := screenshot.NumActiveDisplays()
	if *display < 1 || *display > n {
		log.Fatalf("invalid display %d (found %d active displays)", *display, n)
	}
	if *fps <= 0 {
		log.Fatal("fps must be greater than 0")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	conn, err := dial(ctx, *addr)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	go func() {
		<-ctx.Done()
		conn.Close()
	}()
	log.Printf("connected to receiver %s, streaming display %d", *addr, *display)

	// The receiver never sends data; a read returning means it hung up.
	done := make(chan struct{})
	go func() {
		io.Copy(io.Discard, conn)
		close(done)
	}()

	capture(ctx, done, conn, *display-1, *fps, *quality)
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

// capture grabs the display at a fixed rate and sends each frame.
func capture(ctx context.Context, done <-chan struct{}, conn net.Conn, display, fps, quality int) {
	ticker := time.NewTicker(time.Second / time.Duration(fps))
	defer ticker.Stop()

	w := bufio.NewWriter(conn)
	opts := &jpeg.Options{Quality: quality}
	var buf bytes.Buffer
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
		}

		img, err := screenshot.CaptureDisplay(display)
		if err != nil {
			log.Printf("capture: %v", err)
			continue
		}
		buf.Reset()
		if err := jpeg.Encode(&buf, img, opts); err != nil {
			log.Printf("encode: %v", err)
			continue
		}
		if err := protocol.WriteMessage(w, protocol.MsgFrame, buf.Bytes()); err != nil {
			return
		}
		if err := w.Flush(); err != nil {
			return
		}
	}
}
