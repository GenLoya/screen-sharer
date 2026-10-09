//go:build windows

package proc

import (
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// groupSys holds a Job Object configured to kill every process in it when its
// last handle closes, so the tree also dies if this process crashes.
type groupSys struct {
	once sync.Once
	job  windows.Handle
}

func (g *Group) start() error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return fmt.Errorf("create job object: %w", err)
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return fmt.Errorf("configure job object: %w", err)
	}

	if err := g.cmd.Start(); err != nil {
		windows.CloseHandle(job)
		return err
	}
	g.sys.job = job

	// Processes spawned after this point inherit the job. A shim needs far
	// longer to launch its target than this call takes.
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(g.cmd.Process.Pid))
	if err != nil {
		g.kill()
		return fmt.Errorf("open process: %w", err)
	}
	defer windows.CloseHandle(h)
	if err := windows.AssignProcessToJobObject(job, h); err != nil {
		g.kill()
		return fmt.Errorf("assign process to job: %w", err)
	}
	return nil
}

func (g *Group) kill() {
	g.sys.once.Do(func() {
		windows.TerminateJobObject(g.sys.job, 1)
		windows.CloseHandle(g.sys.job)
		g.cmd.Process.Kill() // in case it never made it into the job
	})
}
