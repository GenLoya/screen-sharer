package video

import (
	"io"
	"os/exec"
	"testing"
	"time"
)

// TestPipelineLatency encodes synthetic frames and decodes them with the same
// arguments the sender and receiver use, checking that the first frame comes
// out quickly. It is skipped when ffmpeg is not installed.
func TestPipelineLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ffmpeg integration test in short mode")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}

	for _, codec := range []Codec{H264, HEVC} {
		t.Run(string(codec), func(t *testing.T) {
			const w, h, fps = 640, 360, 30

			encoder := codec.DefaultEncoder()
			enc := exec.Command(ffmpeg, EncoderArgs(EncoderConfig{
				Input:  RawInput(w, h, fps),
				Filter: SoftwareFilter(w, h, encoder),
				Codec:  codec, Encoder: encoder, FPS: fps, Bitrate: "2M",
			})...)
			dec := exec.Command(ffmpeg, DecoderArgs(codec)...)

			encIn, _ := enc.StdinPipe()
			encOut, _ := enc.StdoutPipe()
			dec.Stdin = encOut
			decOut, _ := dec.StdoutPipe()
			if err := enc.Start(); err != nil {
				t.Fatal(err)
			}
			if err := dec.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() {
				enc.Process.Kill()
				dec.Process.Kill()
				enc.Wait()
				dec.Wait()
			}()

			start := time.Now()
			go func() {
				frame := make([]byte, w*h*4)
				ticker := time.NewTicker(time.Second / fps)
				defer ticker.Stop()
				for i := 0; i < fps*5; i++ {
					<-ticker.C
					for j := range frame {
						frame[j] = byte(i + j)
					}
					if _, err := encIn.Write(frame); err != nil {
						return
					}
				}
			}()

			out := make([]byte, w*h*4)
			firstDone := make(chan time.Duration, 1)
			go func() {
				if _, err := io.ReadFull(decOut, out); err == nil {
					firstDone <- time.Since(start)
				}
			}()

			select {
			case d := <-firstDone:
				t.Logf("first frame after %s", d.Round(time.Millisecond))
				if d > time.Second {
					t.Errorf("first frame took %s; want under 1s", d)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("no frame decoded within 5s")
			}
		})
	}
}
