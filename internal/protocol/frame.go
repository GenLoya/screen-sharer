// Package protocol defines the wire format shared by the sender and receiver.
//
// Each message is encoded as a 1-byte type, a 4-byte big-endian payload
// length, and the payload itself.
package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// MaxPayloadSize guards against corrupt or malicious length prefixes.
const MaxPayloadSize = 32 << 20 // 32 MB

// MsgType identifies the kind of message on the wire.
type MsgType byte

const (
	// MsgHello is the first message from the sender. Payload: JSON Hello.
	MsgHello MsgType = iota + 1
	// MsgVideo carries a chunk of the raw encoded bitstream (sender -> receiver).
	// Chunks do not align with frames; the receiver feeds them to the decoder in order.
	MsgVideo
	// MsgCursor carries the mouse pointer position (sender -> receiver).
	// Payload: Cursor encoded with EncodeCursor.
	MsgCursor
)

// Cursor is the mouse pointer position in frame pixels.
type Cursor struct {
	X, Y    int32
	Visible bool // false when the pointer is outside the streamed display
}

// EncodeCursor serializes c as a 9-byte MsgCursor payload.
func EncodeCursor(c Cursor) []byte {
	p := make([]byte, 9)
	binary.BigEndian.PutUint32(p[0:], uint32(c.X))
	binary.BigEndian.PutUint32(p[4:], uint32(c.Y))
	if c.Visible {
		p[8] = 1
	}
	return p
}

// DecodeCursor parses a MsgCursor payload.
func DecodeCursor(p []byte) (Cursor, error) {
	if len(p) != 9 {
		return Cursor{}, fmt.Errorf("invalid cursor: expected 9 bytes, got %d", len(p))
	}
	return Cursor{
		X:       int32(binary.BigEndian.Uint32(p[0:])),
		Y:       int32(binary.BigEndian.Uint32(p[4:])),
		Visible: p[8] == 1,
	}, nil
}

// Hello describes the stream that follows.
type Hello struct {
	Codec  string `json:"codec"`  // ffmpeg raw format: h264 or hevc
	Width  int    `json:"width"`  // decoded frame width
	Height int    `json:"height"` // decoded frame height
}

// EncodeHello serializes h as a MsgHello payload.
func EncodeHello(h Hello) ([]byte, error) {
	return json.Marshal(h)
}

// DecodeHello parses a MsgHello payload.
func DecodeHello(p []byte) (Hello, error) {
	var h Hello
	if err := json.Unmarshal(p, &h); err != nil {
		return Hello{}, fmt.Errorf("invalid hello: %w", err)
	}
	if h.Width <= 0 || h.Height <= 0 || h.Width > 16384 || h.Height > 16384 {
		return Hello{}, fmt.Errorf("invalid hello: bad size %dx%d", h.Width, h.Height)
	}
	return h, nil
}

// WriteMessage writes a typed, length-prefixed message to w.
func WriteMessage(w io.Writer, t MsgType, payload []byte) error {
	if len(payload) > MaxPayloadSize {
		return fmt.Errorf("payload too large: %d bytes", len(payload))
	}
	var header [5]byte
	header[0] = byte(t)
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

// ReadMessage reads a single typed, length-prefixed message from r.
func ReadMessage(r io.Reader) (MsgType, []byte, error) {
	var header [5]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, err
	}
	size := binary.BigEndian.Uint32(header[1:])
	if size > MaxPayloadSize {
		return 0, nil, fmt.Errorf("payload too large: %d bytes", size)
	}
	payload := make([]byte, size)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return MsgType(header[0]), payload, nil
}
