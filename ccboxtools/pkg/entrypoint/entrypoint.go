// Package entrypoint is the container's fail-closed boot: verify the container's
// identity and egress wall, update the CLIs and GUI app, then exec the
// container's command. Any check failure refuses the container to start.
package entrypoint

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/s12chung/ccbox/ccboxtools/pkg/guiapp"
	"github.com/s12chung/ccbox/ccboxtools/pkg/install"
	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/ccboxtools/pkg/vnc"
	"github.com/s12chung/ccbox/ccboxtools/pkg/vnc/vncdeps"
)

// The unprivileged container identity, matching the image's useradd.
const (
	ccboxUID  = 1000
	ccboxUser = "ccbox"
)

// Run execs argv, replacing this process so the container's command keeps PID 1.
func Run(argv []string, vncInfo *pkginfo.VNCInfo) error {
	if err := checkIdentity(os.Getuid(), currentUserName(), exec.LookPath); err != nil {
		return err
	}
	if os.Getenv(proxyEnv) != "" {
		if err := probeWall(); err != nil {
			return err
		}
	}
	if os.Getenv(pkginfo.EnvVar) != "" {
		if err := install.FromEnv(); err != nil {
			return err
		}
	}
	if vncInfo != nil {
		if err := serveDesktop(vncInfo); err != nil {
			return err
		}
	}
	return execArgv(argv)
}

// serveDesktop starts the image's desktop for the session and installs its GUI
// app — a VNC run without one serves the desktop alone.
func serveDesktop(vncInfo *pkginfo.VNCInfo) error {
	if err := vncdeps.CheckPlaywright(os.Getenv, filepath.Glob); err != nil {
		return err
	}
	if err := vnc.StartDesktop(vncInfo.Config.Resolution); err != nil {
		return err
	}
	if vncInfo.GUIApp != nil {
		if err := guiapp.Install(*vncInfo.GUIApp); err != nil {
			return err
		}
	}
	return nil
}

// execArgv execs argv with the shell `exec`'s semantics — the process is replaced
// in place, PID 1 and its TTY preserved.
func execArgv(argv []string) error {
	if len(argv) == 0 {
		return errors.New("no command to exec")
	}
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return fmt.Errorf("exec %s: %w", argv[0], err)
	}
	//nolint:gosec // the entrypoint's job: exec the container's command, like the shell's `exec "$@"`
	return syscall.Exec(path, append([]string{path}, argv[1:]...), os.Environ())
}
