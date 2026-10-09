// Package proc starts child processes so that they, and anything they spawn,
// never outlive their owner.
//
// This matters for ffmpeg installed through package-manager shims (e.g. scoop
// on Windows): killing the shim does not kill the real ffmpeg it launched.
package proc

import "os/exec"

// Group is a started command together with all of its descendants.
type Group struct {
	cmd *exec.Cmd
	sys groupSys
}

// Start starts cmd inside a new process group.
func Start(cmd *exec.Cmd) (*Group, error) {
	g := &Group{cmd: cmd}
	if err := g.start(); err != nil {
		return nil, err
	}
	return g, nil
}

// Kill terminates the command and all of its descendants. It is safe to call
// more than once. The caller must still call Wait on the command.
func (g *Group) Kill() {
	g.kill()
}
