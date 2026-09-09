// Package entrypoint is the container's fail-closed boot: it verifies the
// container's identity and egress wall, updates the coding CLI, then execs the
// container's command. Any check failure refuses the container to start.
package entrypoint

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/s12chung/ccbox/ccboxtools/pkg/install"
	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
)

// The unprivileged container identity, matching the image's useradd.
const (
	ccboxUID  = 1000
	ccboxUser = "ccbox"
)

// Run verifies the container's identity and egress wall, updates the coding CLI,
// then execs argv — replacing this process, so the container's command keeps PID 1.
// Every check fails closed: one failure and the container refuses to start.
func Run(argv []string) error {
	if err := checkIdentity(os.Getuid(), currentUserName(), exec.LookPath); err != nil {
		return err
	}
	if err := checkGitMountRO(gitConfigMount, linuxMountinfoPath); err != nil {
		return err
	}
	if os.Getenv(proxyEnv) != "" {
		if err := probeWall(); err != nil {
			return err
		}
	}
	if os.Getenv(pkginfo.EnvVar) != "" {
		if err := install.RunFromEnv(); err != nil {
			return err
		}
	}
	return execArgv(argv)
}

// execArgv replaces this process with argv, preserving PID 1 and its TTY — the
// semantics of the shell `exec` this package replaces. Empty argv fails loudly.
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
