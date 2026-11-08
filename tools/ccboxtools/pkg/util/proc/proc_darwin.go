//go:build darwin

package proc

import "syscall"

// dup dups from onto to — darwin has no dup3, so no flags to pass.
func dup(from, to int) error {
	return syscall.Dup2(from, to)
}
