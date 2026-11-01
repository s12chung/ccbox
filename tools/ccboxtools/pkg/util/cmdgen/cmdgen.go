// Package cmdgen spawns fixed binaries as long-lived children: wired to this
// process's stdio and, on request, a passwd user. The arg lists are the
// caller's policy and live with the caller.
package cmdgen

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

// Stdio returns bin's long-lived command, wired to this process's stdio — the
// child speaks through the container's tty and logs.
func Stdio(ctx context.Context, bin string, args ...string) *exec.Cmd {
	//nolint:gosec // the caller's own fixed binary and args
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd
}

// AsUser sets cmd to spawn as the passwd user name — the image's baked
// identities, so never a negative uid.
func AsUser(cmd *exec.Cmd, name string) (*exec.Cmd, error) {
	u, err := user.Lookup(name)
	if err != nil {
		return cmd, fmt.Errorf("user %s: %w", name, err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return cmd, fmt.Errorf("user %s uid: %w", name, err)
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return cmd, fmt.Errorf("user %s gid: %w", name, err)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		//nolint:gosec // G115: a passwd entry is never a negative
		Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)},
	}
	return cmd, nil
}
