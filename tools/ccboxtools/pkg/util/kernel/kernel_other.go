//go:build !linux

package kernel

import (
	"context"
	"errors"
)

// mount refuses off linux: mount(2) is a linux syscall.
func mount(string, uintptr, string) error {
	return errors.New("mount requires linux")
}

// unshareUserNS refuses off linux: user namespaces are a linux kernel feature.
func unshareUserNS(context.Context) (bool, error) {
	return false, errors.New("unshare requires linux")
}

// cloneUserNS refuses off linux: the probes are linux syscalls.
func cloneUserNS() (bool, error) {
	return false, errors.New("clone requires linux")
}

// clone3UserNS refuses off linux: the probes are linux syscalls.
func clone3UserNS() (bool, error) {
	return false, errors.New("clone3 requires linux")
}

// ioUringSetup refuses off linux: the probes are linux syscalls.
func ioUringSetup() (bool, error) {
	return false, errors.New("io_uring requires linux")
}
