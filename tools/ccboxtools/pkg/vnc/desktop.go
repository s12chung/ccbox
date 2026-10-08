// Package vnc starts the image's desktop for the terminal session a run's env
// asks for (pkginfo.VNCConfigEnvVar).
package vnc

import (
	"fmt"
	"os/exec"
)

// desktopBin is the image's desktop launcher: present = the image serves a GUI over VNC.
const desktopBin = "desktop"

// StartDesktop starts the image's desktop in the background, erroring when the
// image ships no desktop or the start fails.
func StartDesktop() error {
	if _, err := exec.LookPath(desktopBin); err != nil {
		return fmt.Errorf("vnc requested: %w", err)
	}
	//nolint:gosec,noctx // the image's own desktop launcher, like the shell's `desktop &` — it outlives any context
	cmd := exec.Command(desktopBin)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("vnc requested: start %s: %w", desktopBin, err)
	}
	// Reaps a desktop that dies before the exec; past it, the exiting container
	// reaps the stack.
	go func() { _ = cmd.Wait() }()
	return nil
}
