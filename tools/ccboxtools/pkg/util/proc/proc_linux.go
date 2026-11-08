//go:build linux

package proc

import "syscall"

// dup dups from onto to — dup3, whose flags this never needs.
func dup(from, to int) error {
	return syscall.Dup3(from, to, 0)
}
