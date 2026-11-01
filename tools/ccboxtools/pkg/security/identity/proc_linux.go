//go:build linux

package identity

import "syscall"

// mountProcfs is mountProc's real self on linux.
func mountProcfs(target string, flags uintptr, data string) error {
	return syscall.Mount("", target, "", flags, data)
}
