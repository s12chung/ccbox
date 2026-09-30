package entrypoint

import (
	"context"
	"os/exec"

	"github.com/s12chung/ccbox/ccboxtools/pkg/util/log"
)

const (
	// desktopBin is the image's desktop launcher: present = the image serves a GUI over VNC.
	desktopBin = "desktop"
)

// startDesktop starts the image's desktop, if it ships one, in the background — the
// script logs itself to the container's scratch, keeping this process's streams for
// the session it serves.
func startDesktop(lookPath func(string) (string, error)) bool {
	if _, err := lookPath(desktopBin); err != nil {
		return false
	}
	//nolint:gosec // the image's own desktop launcher, like the shell's `desktop &`
	cmd := exec.CommandContext(context.Background(), desktopBin)
	if err := cmd.Start(); err != nil {
		log.WarnErr("start desktop", err)
		return false
	}
	// Reaps a desktop that dies before the exec; past it, the exiting container
	// reaps the stack.
	go func() { _ = cmd.Wait() }()
	return true
}
