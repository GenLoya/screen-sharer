//go:build !windows

package proc

import (
	"sync"
	"syscall"
)

// groupSys runs the command as the leader of its own process group.
type groupSys struct {
	once sync.Once
}

func (g *Group) start() error {
	if g.cmd.SysProcAttr == nil {
		g.cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	g.cmd.SysProcAttr.Setpgid = true
	return g.cmd.Start()
}

func (g *Group) kill() {
	g.sys.once.Do(func() {
		syscall.Kill(-g.cmd.Process.Pid, syscall.SIGKILL)
	})
}
