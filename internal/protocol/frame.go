// Package protocol defines the wire format shared by the sender and receiver.
//
// Each message is encoded as a 1-byte type, a 4-byte big-endian payload
// length, and the payload itself.
package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

// MaxPayloadSize guards against corrupt or malicious length prefixes.
const MaxPayloadSize = 32 << 20 // 32 MB

// MsgType identifies the kind of message on the wire.
type MsgType byte

const (
	// MsgFrame carries one JPEG-encoded screen frame (sender -> receiver).
	MsgFrame MsgType = iota + 1
)

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
