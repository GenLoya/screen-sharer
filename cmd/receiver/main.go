// Command receiver listens for senders and displays the stream in a window.
//
// Only one sender is shown at a time: when a new sender connects, the previous
// one is disconnected. The video is decoded by an ffmpeg process per sender.
//
// Press F11, F or double-click to toggle fullscreen; Esc leaves fullscreen.
package main

import (
	"bufio"
	"flag"
	"log"
	"net"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"genloya/screen-sharer/internal/protocol"
	"genloya/screen-sharer/internal/video"
)

// doubleClickWindow is the maximum gap between clicks of a double-click.
const doubleClickWindow = 400 * time.Millisecond

func main() {
	addr := flag.String("addr", ":9000", "address to listen on for senders")
	fullscreen := flag.Bool("fullscreen", false, "start in fullscreen mode")
	ffmpegName := flag.String("ffmpeg", "ffmpeg", "ffmpeg executable name or path")
	flag.Parse()

	ffmpeg, err := video.FindFFmpeg(*ffmpegName)
	if err != nil {
		log.Fatal(err)
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	log.Printf("waiting for senders on %s", ln.Addr())

	g := &game{addr: ln.Addr().String(), ffmpeg: ffmpeg}
	go g.accept(ln)

	ebiten.SetWindowTitle("Screen Receiver")
	ebiten.SetWindowSize(1280, 720)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetFullscreen(*fullscreen)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}

type game struct {
	addr   string
	ffmpeg string

	mu      sync.Mutex
	current net.Conn
	pending *rawFrame
	cursor  protocol.Cursor

	frame     *ebiten.Image
	cursorImg *ebiten.Image
	lastClick time.Time
}

func (g *game) accept(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			return
		}
		g.replace(conn)
	}
}

// replace makes conn the active sender and disconnects the previous one.
func (g *game) replace(conn net.Conn) {
	g.mu.Lock()
	old := g.current
	g.current = conn
	g.cursor = protocol.Cursor{}
	g.mu.Unlock()

	if old != nil {
		log.Printf("sender %s replaced by %s", old.RemoteAddr(), conn.RemoteAddr())
		old.Close()
	} else {
		log.Printf("sender connected: %s", conn.RemoteAddr())
	}
	go g.serve(conn)
}

func (g *game) serve(conn net.Conn) {
	defer g.drop(conn)

	var dec *decoder
	defer func() {
		if dec != nil {
			dec.Close()
		}
	}()

	r := bufio.NewReader(conn)
	for {
		t, payload, err := protocol.ReadMessage(r)
		if err != nil {
			return
		}
		switch t {
		case protocol.MsgHello:
			if dec != nil {
				log.Print("duplicate hello from sender")
				return
			}
			hello, err := protocol.DecodeHello(payload)
			if err != nil {
				log.Print(err)
				return
			}
			start := time.Now()
			var first sync.Once
			dec, err = startDecoder(g.ffmpeg, hello, func(f rawFrame) {
				first.Do(func() { log.Printf("first frame decoded after %s", time.Since(start).Round(time.Millisecond)) })
				g.setPending(conn, f)
			})
			if err != nil {
				log.Printf("decoder: %v", err)
				return
			}
			log.Printf("receiving %s %dx%d from %s", hello.Codec, hello.Width, hello.Height, conn.RemoteAddr())
		case protocol.MsgVideo:
			if dec == nil {
				continue
			}
			if _, err := dec.Write(payload); err != nil {
				log.Printf("decoder: %v", err)
				return
			}
		case protocol.MsgCursor:
			c, err := protocol.DecodeCursor(payload)
			if err != nil {
				log.Print(err)
				continue
			}
			g.mu.Lock()
			if g.current == conn {
				g.cursor = c
			}
			g.mu.Unlock()
		}
	}
}

// setPending stores the latest frame from conn, if it is still the active sender.
func (g *game) setPending(conn net.Conn, f rawFrame) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.current != conn {
		putFrameBuf(f.pix)
		return
	}
	if g.pending != nil {
		putFrameBuf(g.pending.pix)
	}
	g.pending = &f
}

func (g *game) drop(conn net.Conn) {
	conn.Close()

	g.mu.Lock()
	defer g.mu.Unlock()
	if g.current == conn {
		g.current = nil
		g.cursor = protocol.Cursor{}
		log.Printf("sender disconnected: %s", conn.RemoteAddr())
	}
}

func (g *game) Update() error {
	g.handleFullscreen()

	g.mu.Lock()
	f := g.pending
	g.pending = nil
	g.mu.Unlock()

	if f != nil {
		if g.frame == nil || g.frame.Bounds().Dx() != f.width || g.frame.Bounds().Dy() != f.height {
			if g.frame != nil {
				g.frame.Deallocate()
			}
			g.frame = ebiten.NewImage(f.width, f.height)
		}
		g.frame.WritePixels(f.pix)
		putFrameBuf(f.pix)
	}
	return nil
}

// handleFullscreen toggles fullscreen on F11, F or double-click, and leaves it on Esc.
func (g *game) handleFullscreen() {
	toggle := inpututil.IsKeyJustPressed(ebiten.KeyF11) || inpututil.IsKeyJustPressed(ebiten.KeyF)

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		now := time.Now()
		if now.Sub(g.lastClick) <= doubleClickWindow {
			toggle = true
			g.lastClick = time.Time{}
		} else {
			g.lastClick = now
		}
	}

	switch {
	case toggle:
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape) && ebiten.IsFullscreen():
		ebiten.SetFullscreen(false)
	}
}

func (g *game) Draw(screen *ebiten.Image) {
	g.mu.Lock()
	connected := g.current != nil
	cursor := g.cursor
	g.mu.Unlock()

	if g.frame != nil {
		fit := fitTransform(screen, g.frame)
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		op.GeoM = fit
		screen.DrawImage(g.frame, op)

		if connected && cursor.Visible {
			if g.cursorImg == nil {
				g.cursorImg = newCursorImage()
			}
			x, y := fit.Apply(float64(cursor.X), float64(cursor.Y))
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(x, y)
			screen.DrawImage(g.cursorImg, op)
		}
	}
	if !connected {
		ebitenutil.DebugPrint(screen, "Waiting for a sender on "+g.addr+"...\nF11 / double-click: fullscreen")
	}
}

func (g *game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}

// fitTransform scales src to fit dst while preserving its aspect ratio, centered.
func fitTransform(dst, src *ebiten.Image) ebiten.GeoM {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := dst.Bounds().Dx(), dst.Bounds().Dy()
	scale := min(float64(dw)/float64(sw), float64(dh)/float64(sh))

	var m ebiten.GeoM
	m.Scale(scale, scale)
	m.Translate((float64(dw)-float64(sw)*scale)/2, (float64(dh)-float64(sh)*scale)/2)
	return m
}
