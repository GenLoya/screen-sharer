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
		if err := WriteMessage(&buf, MsgVideo, p); err != nil {
			t.Fatalf("WriteMessage: %v", err)
		}
	}
	for i, want := range payloads {
		gotType, got, err := ReadMessage(&buf)
		if err != nil {
			t.Fatalf("ReadMessage %d: %v", i, err)
		}
		if gotType != MsgVideo || !bytes.Equal(got, want) {
			t.Fatalf("message %d mismatch: got type %d (%d bytes), want type %d (%d bytes)",
				i, gotType, len(got), MsgVideo, len(want))
		}
	}
}

func TestReadMessageRejectsOversize(t *testing.T) {
	var header [5]byte
	header[0] = byte(MsgVideo)
	binary.BigEndian.PutUint32(header[1:], MaxPayloadSize+1)
	if _, _, err := ReadMessage(bytes.NewReader(header[:])); err == nil {
		t.Fatal("expected error for oversize payload")
	}
}

func TestCursorRoundTrip(t *testing.T) {
	for _, want := range []Cursor{{X: 100, Y: 200, Visible: true}, {X: -5, Y: 0}} {
		got, err := DecodeCursor(EncodeCursor(want))
		if err != nil || got != want {
			t.Fatalf("DecodeCursor = %+v, %v; want %+v", got, err, want)
		}
	}
	if _, err := DecodeCursor([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected error for short payload")
	}
}

func TestHelloRoundTrip(t *testing.T) {
	want := Hello{Codec: "hevc", Width: 1920, Height: 1080}
	p, err := EncodeHello(want)
	if err != nil {
		t.Fatalf("EncodeHello: %v", err)
	}
	got, err := DecodeHello(p)
	if err != nil || got != want {
		t.Fatalf("DecodeHello = %+v, %v; want %+v", got, err, want)
	}
	if _, err := DecodeHello([]byte(`{"codec":"h264","width":0,"height":1080}`)); err == nil {
		t.Fatal("expected error for zero width")
	}
}
