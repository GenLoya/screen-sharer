// Command receiver listens for senders and displays the stream in a window.
//
// Only one sender is shown at a time: when a new sender connects, the previous
// one is disconnected.
//
// Press F11, F or double-click to toggle fullscreen; Esc leaves fullscreen.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"image"
	"image/jpeg"
	"log"
	"net"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"genloya/screen-sharer/internal/protocol"
)

// doubleClickWindow is the maximum gap between clicks of a double-click.
const doubleClickWindow = 400 * time.Millisecond

func main() {
	addr := flag.String("addr", ":9000", "address to listen on for senders")
	fullscreen := flag.Bool("fullscreen", false, "start in fullscreen mode")
	flag.Parse()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	log.Printf("waiting for senders on %s", ln.Addr())

	g := &game{addr: ln.Addr().String()}
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
	addr string

	mu      sync.Mutex
	current net.Conn
	pending image.Image

	frame     *ebiten.Image
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

	r := bufio.NewReader(conn)
	for {
		t, payload, err := protocol.ReadMessage(r)
		if err != nil {
			return
		}
		if t != protocol.MsgFrame {
			continue
		}
		img, err := jpeg.Decode(bytes.NewReader(payload))
		if err != nil {
			log.Printf("decode: %v", err)
			continue
		}
		g.mu.Lock()
		if g.current == conn {
			g.pending = img
		}
		g.mu.Unlock()
	}
}

func (g *game) drop(conn net.Conn) {
	conn.Close()

	g.mu.Lock()
	defer g.mu.Unlock()
	if g.current == conn {
		g.current = nil
		log.Printf("sender disconnected: %s", conn.RemoteAddr())
	}
}

func (g *game) Update() error {
	g.handleFullscreen()

	g.mu.Lock()
	img := g.pending
	g.pending = nil
	g.mu.Unlock()

	if img != nil {
		if g.frame != nil {
			g.frame.Deallocate()
		}
		g.frame = ebiten.NewImageFromImage(img)
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
	g.mu.Unlock()

	if g.frame != nil {
		drawFit(screen, g.frame)
	}
	if !connected {
		ebitenutil.DebugPrint(screen, "Waiting for a sender on "+g.addr+"...\nF11 / double-click: fullscreen")
	}
}

func (g *game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return outsideWidth, outsideHeight
}

// drawFit scales src to fit dst while preserving its aspect ratio.
func drawFit(dst, src *ebiten.Image) {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := dst.Bounds().Dx(), dst.Bounds().Dy()
	scale := min(float64(dw)/float64(sw), float64(dh)/float64(sh))

	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate((float64(dw)-float64(sw)*scale)/2, (float64(dh)-float64(sh)*scale)/2)
	dst.DrawImage(src, op)
}
