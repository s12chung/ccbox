//go:build !linux

package cbpf

import "errors"

// syscallNrs refuses off linux: the wire numbers are linux syscalls.
func syscallNrs([]string) ([]int, error) {
	return nil, errors.New("syscall numbers require linux")
}
