package video

import (
	"slices"
	"testing"
)

func TestParseCodec(t *testing.T) {
	cases := map[string]Codec{"h264": H264, "AVC": H264, "h265": HEVC, "HEVC": HEVC}
	for in, want := range cases {
		got, err := ParseCodec(in)
		if err != nil || got != want {
			t.Errorf("ParseCodec(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := ParseCodec("vp9"); err == nil {
		t.Error("expected error for unsupported codec")
	}
}

func TestEvenSize(t *testing.T) {
	w, h := EvenSize(1921, 1081)
	if w != 1920 || h != 1080 {
		t.Fatalf("EvenSize = %dx%d; want 1920x1080", w, h)
	}
}

func TestEncoderArgs(t *testing.T) {
	args := EncoderArgs(EncoderConfig{
		Input:  RawInput(1921, 1080, 30),
		Filter: SoftwareFilter(1921, 1080, "libx265"),
		Codec:  HEVC, Encoder: "libx265", FPS: 30, Bitrate: "4M",
	})
	for _, want := range []string{"1921x1080", "crop=1920:1080:0:0,format=yuv420p", "libx265", "zerolatency", "hevc"} {
		if !slices.Contains(args, want) {
			t.Errorf("encoder args missing %q: %v", want, args)
		}
	}
	if args[len(args)-1] != "pipe:1" {
		t.Errorf("encoder must write to stdout, got %q", args[len(args)-1])
	}
}
