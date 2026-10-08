package openwith

import (
	"errors"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type processTree struct {
	group uint32

	mu  sync.Mutex
	job windows.Handle
}

func prepare(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_SUSPENDED
}

func attach(cmd *exec.Cmd) (*processTree, error) {
	pid := uint32(cmd.Process.Pid)

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("creating a job object: %w", err)
	}
	if err := assign(job, pid); err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("adding the shell to a job object: %w", err)
	}
	if err := resume(pid); err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("resuming the shell: %w", err)
	}

	return &processTree{group: pid, job: job}, nil
}

func assign(job windows.Handle, pid uint32) error {
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(process)

	return windows.AssignProcessToJobObject(job, process)
}

func resume(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)

	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	resumed := 0
	next := windows.Thread32First(snapshot, &entry)
	for ; next == nil; next = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		if err := resumeThread(entry.ThreadID); err != nil {
			return err
		}
		resumed++
	}
	if !errors.Is(next, windows.ERROR_NO_MORE_FILES) {
		return next
	}
	if resumed == 0 {
		return errors.New("the shell has no thread to resume")
	}

	return nil
}

func resumeThread(id uint32) error {
	thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, id)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(thread)

	_, err = windows.ResumeThread(thread)
	return err
}

func (t *processTree) terminate() error {
	return windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, t.group)
}

func (t *processTree) kill() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.job == 0 {
		return nil
	}
	return windows.TerminateJobObject(t.job, 1)
}

func (t *processTree) release() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.job != 0 {
		_ = windows.CloseHandle(t.job)
		t.job = 0
	}
}
