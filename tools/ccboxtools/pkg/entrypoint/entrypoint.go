// Package entrypoint is the container's fail-closed boot: bring up and prove
// the egress wall, install the env's CLI once, then idle as the supervisor —
// no workload ever runs at boot. Sessions prove themselves before exec'ing
// anything: no-proxy's ccbox arrival, the egress wall's nsenter-then-drop.
package entrypoint

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/guiapp"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/install"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/identity"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/proxy"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/supervise"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/vnc"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/vnc/vncdeps"
)

// Run boots the container: the egress wall's boot when the proxy env is
// present, the plain boot otherwise — the container's `docker run`, with no
// workload in the run.
func Run(ctx context.Context, vncInfo *pkginfo.VNCInfo) error {
	isProxy := proxy.Proxied()
	if err := identity.Verify(identity.Opts{IsProxy: isProxy}); err != nil {
		return err
	}
	if isProxy {
		if vncInfo != nil {
			return errors.New("vnc is incompatible with the egress wall")
		}
		return supervise.RunF(ctx, runProxy)
	}
	if err := RunMain(ctx, vncInfo); err != nil {
		return err
	}
	supervise.Run(ctx) // the boot's idle
	return nil
}

// runProxy brings the egress wall up and proves it with a one-shot proxy child
// — which runs the boot's main work on its way out — leaving tinyproxy as the
// only supervisee.
func runProxy(ctx context.Context) ([]*supervise.Respawnable, error) {
	var tproxy *supervise.Respawnable
	err := proxy.CreateNetwork(ctx, func(_ context.Context, spawn proxy.Spawn) (proxy.PID, error) {
		tproxy = &supervise.Respawnable{Name: "tinyproxy", Spawn: spawn}
		if err := tproxy.Start(ctx); err != nil {
			return nil, err
		}
		return tproxy.PID, nil
	})
	if err != nil {
		return nil, err
	}
	if err := proxy.ValidateWithChild(ctx); err != nil {
		return nil, err
	}
	// the boot's readiness marker
	log.Info("egress wall: ready")
	return []*supervise.Respawnable{tproxy}, nil
}

// RunChild is the proxy child's landing: prove the egress wall from inside,
// go ready, then run the boot's main work.
func RunChild(ctx context.Context) error {
	return proxy.ValidateAndRun(ctx, func(ctx context.Context) error {
		return RunMain(ctx, nil) // no VNC with proxy
	})
}

// RunMain is a boot's main work: install the env's CLI and serve the desktop
// when the run asks for one — the no-proxy boot runs it after its arrival
// proof, the proxy child after going ready.
func RunMain(ctx context.Context, vncInfo *pkginfo.VNCInfo) error {
	if os.Getenv(pkginfo.EnvVar) != "" {
		if err := install.FromEnv(ctx); err != nil {
			return err
		}
	}
	if vncInfo != nil {
		// the desktop is served unattended: its death doesn't take the boot down
		if err := serveDesktop(ctx, vncInfo); err != nil {
			return err
		}
	}
	return nil
}

// serveDesktop starts the image's desktop and installs its GUI app — a VNC run
// without one serves the desktop alone.
func serveDesktop(ctx context.Context, vncInfo *pkginfo.VNCInfo) error {
	if err := vncdeps.CheckPlaywright(os.Getenv, filepath.Glob); err != nil {
		return err
	}
	if err := vnc.StartDesktop(ctx); err != nil {
		return err
	}
	if vncInfo.GUIApp != nil {
		if err := guiapp.Install(ctx, *vncInfo.GUIApp); err != nil {
			return err
		}
	}
	return nil
}

// Exec is the docker exec target: a session proves its arrival before
// anything runs.
func Exec(ctx context.Context, argv []string) error {
	if len(argv) == 0 {
		return errors.New("no command to exec")
	}
	isProxy := proxy.Proxied()
	if err := identity.Verify(identity.Opts{IsProxy: isProxy, IsExec: true}); err != nil {
		return err
	}
	if isProxy {
		// setns needs SYS_ADMIN, so the netns crossing precedes the drop
		return proxy.NSIsolatedExec(ctx, append([]string{"proxy", "exec", "--"}, argv...)...)
	}
	return ExecArgv(argv)
}

// ExecArgv execs argv with the shell `exec`'s semantics — the process is
// replaced in place, its pid and TTY preserved.
func ExecArgv(argv []string) error {
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
