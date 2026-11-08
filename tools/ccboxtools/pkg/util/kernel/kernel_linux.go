//go:build linux

package kernel

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

func mount(target string, flags uintptr, data string) error {
	return syscall.Mount("", target, "", flags, data)
}

func unshareUserNS(ctx context.Context) (bool, error) {
	// plain -U writes no uid_map — with -Ur, a failed map write would read as a
	// userNS denial the profile never made
	cmd := exec.CommandContext(ctx, "unshare", "-U", "true")
	// the C locale pins the denial text the verdict greps
	cmd.Env = []string{"LC_ALL=C", "LANG=C"}
	out, err := cmd.CombinedOutput()
	if err == nil {
		return false, nil
	}
	// the profile's one answer is EPERM — any other denial is an unreadable proof
	if strings.Contains(strings.ToLower(string(out)), "operation not permitted") {
		return true, nil
	}
	return false, fmt.Errorf("unshare -U true: unrecognized denial, want EPERM: %w\n%s", err, out)
}

func cloneUserNS() (bool, error) {
	// s390's clone ABI orders the flags at arg1 — matching the profile's clone
	// rule pair
	a0, a1 := uintptr(unix.CLONE_NEWUSER), uintptr(0)
	if runtime.GOARCH == "s390x" {
		a0, a1 = a1, a0
	}
	r1, _, errno := unix.RawSyscall(unix.SYS_CLONE, a0, a1, 0)
	switch {
	case errno == unix.EPERM:
		return true, nil
	case errno != 0:
		return false, fmt.Errorf("clone(CLONE_NEWUSER): unrecognized denial, want EPERM: %w", errno)
	case r1 == 0: // the kernel hands the forked child 0 — it must not run any Go code
		unix.Exit(0)
	}
	// a surviving child (its tid in r1) means the door let a userns through
	return false, nil
}

func clone3UserNS() (bool, error) {
	var args [8]uint64 // struct clone_args: the flags lead, the rest zero
	args[0] = unix.CLONE_NEWUSER
	//nolint:gosec // the syscall ABI: a pointer sized for the kernel's read
	_, _, errno := unix.RawSyscall(unix.SYS_CLONE3, uintptr(unsafe.Pointer(&args[0])), uintptr(len(args))*8, 0)
	switch errno {
	case 0:
		return false, nil
	case unix.ENOSYS, unix.EPERM:
		return true, nil
	default:
		return false, fmt.Errorf("clone3(CLONE_NEWUSER): unrecognized denial, want ENOSYS: %w", errno)
	}
}

func ioUringSetup() (bool, error) {
	var params [16]uint64 // struct io_uring_params, written on success
	//nolint:gosec // the syscall ABI: a pointer sized for the kernel's write
	r1, _, errno := unix.RawSyscall(unix.SYS_IO_URING_SETUP, 1, uintptr(unsafe.Pointer(&params[0])), 0)
	switch errno {
	case 0:
		_, _, _ = unix.RawSyscall(unix.SYS_CLOSE, r1, 0, 0) // the ring dies with the refused start
		return false, nil
	case unix.ENOSYS:
		return true, nil
	default:
		return false, fmt.Errorf("io_uring_setup: unrecognized denial, want ENOSYS: %w", errno)
	}
}
