package proc

import (
	"io"
	"os/exec"
	"testing"
	"time"
)

// TestKillTerminatesTree kills ffmpeg at different points of its startup and
// checks that no process in the tree survives. Through a package-manager shim
// (e.g. scoop), killing only the shim while it launches the real ffmpeg
// (around 10-20ms in) leaves the real ffmpeg suspended forever.
func TestKillTerminatesTree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ffmpeg integration test in short mode")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}

	for delay := 0; delay <= 40; delay++ {
		cmd := exec.Command(ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "h264", "-i", "pipe:0", "-f", "null", "-")
		stdin, err := cmd.StdinPipe()
		if err != nil {
			t.Fatal(err)
		}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		g, err := Start(cmd)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Duration(delay) * time.Millisecond)

		g.Kill()
		stdin.Close()
		eof := make(chan struct{})
		go func() {
			io.Copy(io.Discard, stdout)
			close(eof)
		}()
		select {
		case <-eof:
		case <-time.After(2 * time.Second):
			t.Errorf("killed after %dms: a process in the tree survived and still holds stdout", delay)
		}
		cmd.Wait()
	}
}
