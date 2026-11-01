// Package vnc starts the image's desktop for the terminal session a run's env
// asks for (pkginfo.VNCConfigEnvVar).
package vnc

import (
	"context"
	"fmt"
	"os/exec"
)

// desktopBin is the image's desktop launcher: present = the image serves a GUI over VNC.
const desktopBin = "desktop"

// StartDesktop starts the image's desktop in the background, erroring when the
// image ships no desktop or the start fails. The desktop is the shell's
// `desktop &`: its ctx cancels only at shutdown, and the dying container takes
// the stack with it.
func StartDesktop(ctx context.Context) error {
	if _, err := exec.LookPath(desktopBin); err != nil {
		return fmt.Errorf("vnc requested: %w", err)
	}
	//nolint:gosec // the image's own desktop launcher
	cmd := exec.CommandContext(ctx, desktopBin)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("vnc requested: start %s: %w", desktopBin, err)
	}
	// Reaps a desktop that dies before the exec; past it, the exiting container
	// reaps the stack.
	go func() { _ = cmd.Wait() }()
	return nil
}
