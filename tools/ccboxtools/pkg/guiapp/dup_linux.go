//go:build linux

package guiapp

import "syscall"

// dupStream dups from onto to — dup3, whose flags this never needs.
func dupStream(from, to int) error {
	return syscall.Dup3(from, to, 0)
}
