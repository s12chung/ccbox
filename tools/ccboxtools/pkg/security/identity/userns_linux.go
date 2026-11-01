//go:build linux

package identity

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// unshareUserNS probes through unshare(1): single-threaded, so the kernel answers
// its unshare(CLONE_NEWUSER) instead of EINVAL-ing this process's threads.
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
