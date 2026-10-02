package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	payloads := [][]byte{[]byte("hello"), {}, bytes.Repeat([]byte{0xAB}, 4096)}

	for _, p := range payloads {
		if err := WriteMessage(&buf, MsgFrame, p); err != nil {
			t.Fatalf("WriteMessage: %v", err)
		}
	}
	for i, want := range payloads {
		gotType, got, err := ReadMessage(&buf)
		if err != nil {
			t.Fatalf("ReadMessage %d: %v", i, err)
		}
		if gotType != MsgFrame || !bytes.Equal(got, want) {
			t.Fatalf("message %d mismatch: got type %d (%d bytes), want type %d (%d bytes)",
				i, gotType, len(got), MsgFrame, len(want))
		}
	}
}

func TestReadMessageRejectsOversize(t *testing.T) {
	var header [5]byte
	header[0] = byte(MsgFrame)
	binary.BigEndian.PutUint32(header[1:], MaxPayloadSize+1)
	if _, _, err := ReadMessage(bytes.NewReader(header[:])); err == nil {
		t.Fatal("expected error for oversize payload")
	}
}
