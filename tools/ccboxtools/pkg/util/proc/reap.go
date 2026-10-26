package proc

import "syscall"

// ReapOne waits out one exited child without blocking; pid <= 0 when none waits.
func ReapOne() (syscall.WaitStatus, int) {
	var ws syscall.WaitStatus
	pid, err := syscall.Wait4(-1, &ws, syscall.WNOHANG, nil)
	if err != nil {
		return ws, -1
	}
	return ws, pid
}
