// Package cmdgen spawns fixed binaries as long-lived children, wired to this
// process's stdio. The arg lists are the caller's policy and live with the
// caller.
package cmdgen

import (
	"context"
	"os"
	"os/exec"
)

// Stdio returns bin's long-lived command, wired to this process's stdio — the
// child speaks through the container's tty and logs.
func Stdio(ctx context.Context, bin string, args ...string) *exec.Cmd {
	//nolint:gosec // the caller's own fixed binary and args
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd
}
