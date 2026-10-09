package video

import "testing"

func TestDDAGrabFilter(t *testing.T) {
	cases := []struct {
		w, h    int
		encoder string
		want    string
	}{
		{1920, 1080, "h264_qsv", "hwmap=derive_device=qsv,format=qsv,vpp_qsv=format=nv12"},
		{1921, 1080, "h264_qsv", "hwdownload,format=bgra,crop=1920:1080:0:0,format=nv12"},
		{1920, 1080, "libx264", "hwdownload,format=bgra,crop=1920:1080:0:0,format=yuv420p"},
	}
	for _, c := range cases {
		if got := DDAGrabFilter(c.w, c.h, c.encoder); got != c.want {
			t.Errorf("DDAGrabFilter(%d, %d, %s) = %q; want %q", c.w, c.h, c.encoder, got, c.want)
		}
	}
}

func TestParseAVFoundationScreens(t *testing.T) {
	out := `[AVFoundation indev @ 0x7f8] AVFoundation video devices:
[AVFoundation indev @ 0x7f8] [0] FaceTime HD Camera
[AVFoundation indev @ 0x7f8] [1] OBS Virtual Camera
[AVFoundation indev @ 0x7f8] [2] Capture screen 0
[AVFoundation indev @ 0x7f8] [3] Capture screen 1
[AVFoundation indev @ 0x7f8] AVFoundation audio devices:
[AVFoundation indev @ 0x7f8] [0] MacBook Pro Microphone
: Input/output error`
	got := ParseAVFoundationScreens(out)
	if len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("ParseAVFoundationScreens = %v; want map[0:2 1:3]", got)
	}
}
