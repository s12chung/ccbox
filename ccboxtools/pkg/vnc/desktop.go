// Package vnc starts the image's desktop for the terminal session it serves,
// at the resolution of the VNC session a run's env asks for
// (pkginfo.VNCConfigEnvVar).
package vnc

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

const (
	// desktopBin is the image's desktop launcher: present = the image serves a GUI over VNC.
	desktopBin = "desktop"

	// vncResolutionEnv is the desktop script's ENV VAR
	vncResolutionEnv = "VNC_RESOLUTION"
)

// StartDesktop starts the image's desktop in the background at the run's
// resolution, erroring when the image ships no desktop or the start fails.
func StartDesktop(resolution string) error {
	if _, err := exec.LookPath(desktopBin); err != nil {
		return fmt.Errorf("vnc requested: %w", err)
	}
	//nolint:gosec,noctx // the image's own desktop launcher, like the shell's `desktop &` — it outlives any context
	cmd := exec.Command(desktopBin)
	// clean out vncResolutionEnv from existing ENV
	env := slices.DeleteFunc(os.Environ(), func(e string) bool {
		return strings.HasPrefix(e, vncResolutionEnv+"=")
	})
	env = append(env, vncResolutionEnv+"="+resolution)
	cmd.Env = env
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("vnc requested: start %s: %w", desktopBin, err)
	}
	// Reaps a desktop that dies before the exec; past it, the exiting container
	// reaps the stack.
	go func() { _ = cmd.Wait() }()
	return nil
}
