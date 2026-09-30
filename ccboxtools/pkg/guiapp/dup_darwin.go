//go:build darwin

package guiapp

import "syscall"

// dupStream dups from onto to — darwin has no dup3, so no flags to pass.
func dupStream(from, to int) error {
	return syscall.Dup2(from, to)
}
