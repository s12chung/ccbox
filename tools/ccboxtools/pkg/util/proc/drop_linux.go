//go:build linux

package proc

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// prctl ops for the drop.
const (
	prCapBsetDrop   = 24
	prSetNoNewPrivs = 38

	capVersion3 = 0x20080522 // capset's v3 header: both data words
	capLast     = 63         // empty the whole bounding set; beyond-kernel bits EINVAL harmlessly
)

// capset's fixed layouts — std syscall carries no capset wrapper.
type capUserHeader struct {
	Version uint32
	Pid     int32
}

type capUserData struct {
	Effective, Permitted, Inheritable uint32
}

// DropTo gives up the process's every privilege in one order — the order is
// the design: each step needs a privilege a later step surrenders.
func DropTo(uid int) error {
	if errno := prctl(prSetNoNewPrivs, 1); errno != 0 { // no later exec can regain a thing
		return fmt.Errorf("set no_new_privs: %w", errno)
	}
	// SETPCAP still funds the prctl
	for c := 0; c <= capLast; c++ {
		errno := prctl(prCapBsetDrop, c)
		if errno != 0 && errno != syscall.EINVAL {
			return fmt.Errorf("drop bounding cap %d: %w", c, errno)
		}
	}
	// the caps still fund the identity drop — gid too, with the supplementary groups cleared
	if err := syscall.Setgroups([]int{}); err != nil {
		return fmt.Errorf("clear groups: %w", err)
	}
	if err := syscall.Setresgid(uid, uid, uid); err != nil {
		return fmt.Errorf("setresgid %d: %w", uid, err)
	}
	if err := syscall.Setresuid(uid, uid, uid); err != nil {
		return fmt.Errorf("setresuid %d: %w", uid, err)
	}
	return clearCaps()
}

// ReadStatus parses this process's own status — the drop's ground truth.
func ReadStatus() (*Status, error) {
	f, err := os.Open("/proc/self/status") //nolint:gosec // the fixed procfs path
	if err != nil {
		return nil, err
	}
	defer func() { log.WarnErr("close status", f.Close()) }()
	return parseStatus(f)
}

func prctl(op, arg int) syscall.Errno {
	_, _, errno := syscall.RawSyscall(syscall.SYS_PRCTL, uintptr(op), uintptr(arg), 0)
	return errno
}

// clearCaps zeroes the inheritable, permitted and effective sets with one
// capset — dropping own caps never needs a capability.
func clearCaps() error {
	hdr := capUserHeader{Version: capVersion3}
	data := [2]capUserData{}
	_, _, errno := syscall.Syscall(syscall.SYS_CAPSET,
		uintptr(unsafe.Pointer(&hdr)), uintptr(unsafe.Pointer(&data[0])), 0) //nolint:gosec // capset's own fixed-size structs
	if errno != 0 {
		return fmt.Errorf("capset: %w", errno)
	}
	return nil
}
