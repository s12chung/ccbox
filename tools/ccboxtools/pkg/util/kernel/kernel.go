// Package kernel is the linux kernel's syscall mechanics: the mount and the
// denial probes the security proofs run — every function refuses off linux.
package kernel

import "context"

// Mount issues mount(2) with the caller's flags and data.
func Mount(target string, flags uintptr, data string) error {
	return mount(target, flags, data)
}

// UnshareUserNS probes through unshare(1): single-threaded, so the kernel answers
// its unshare(CLONE_NEWUSER) instead of EINVAL-ing this process's threads.
func UnshareUserNS(ctx context.Context) (bool, error) {
	return unshareUserNS(ctx)
}

// CloneUserNS probes through raw clone(CLONE_NEWUSER) — the door's clone leg,
// which the unshare(1) probe never exercises.
func CloneUserNS() (bool, error) {
	return cloneUserNS()
}

// Clone3UserNS probes clone3(CLONE_NEWUSER): the overlay's ENOSYS and the
// whitelist's EPERM both read as denied.
func Clone3UserNS() (bool, error) {
	return clone3UserNS()
}

// IoUringSetup probes io_uring — the overlay's ENOSYS fingerprints ccbox's
// rendered profile; an EPERM would mean the engine's default loaded instead.
func IoUringSetup() (bool, error) {
	return ioUringSetup()
}
