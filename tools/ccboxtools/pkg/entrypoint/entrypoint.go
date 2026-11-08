// Package entrypoint is the container's fail-closed boot: bring up and prove the
// egress wall, install the env's CLI once, then idle as the supervisor — no
// workload ever runs at boot; sessions arrive by exec, proven first.
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

// Run boots the container: the egress wall's boot when the proxy env is present,
// the plain boot otherwise.
func Run(ctx context.Context, vncInfo *pkginfo.VNCInfo) error {
	isProxy := proxy.Proxied()
	if err := identity.Verify(ctx, isProxy); err != nil {
		return err
	}
	if isProxy {
		if vncInfo != nil {
			return errors.New("vnc is incompatible with the egress wall")
		}
		if err := identity.HardenProcfs(); err != nil {
			return err
		}
		if err := identity.WriteDropFilter(); err != nil {
			return err
		}
		return supervise.RunF(ctx, runProxy)
	}
	if err := RunMain(ctx, vncInfo); err != nil {
		return err
	}
	supervise.Run(ctx) // the boot's idle
	return nil
}

// runProxy brings the egress wall up, proven by a one-shot proxy child that runs
// the boot's main work on its way out — tinyproxy stays the only supervisee.
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
	log.Info(pkginfo.ReadyMarker)
	return []*supervise.Respawnable{tproxy}, nil
}

// RunChild is the proxy child's landing: prove the egress wall from inside,
// go ready, then run the boot's main work.
func RunChild(ctx context.Context) error {
	return proxy.ValidateAndRun(ctx, func(ctx context.Context) error {
		return RunMain(ctx, nil) // no VNC with proxy
	})
}

// RunMain is a boot's main work: install the env's CLI and serve the desktop on a
// VNC run — the no-proxy boot after its arrival proof, the proxy child after going ready.
func RunMain(ctx context.Context, vncInfo *pkginfo.VNCInfo) error {
	if os.Getenv(pkginfo.EnvVar) != "" {
		if err := install.FromEnv(ctx); err != nil {
			return err
		}
	}
	if vncInfo != nil {
		if err := serveDesktop(ctx, vncInfo); err != nil {
			return err
		}
	}
	log.Info(pkginfo.DevboxReadyMarker)
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

// verifyAndResetDrop proves the drop and resets HOME to it before any exec — a var
// so tests can stub it.
var verifyAndResetDrop = identity.VerifyAndResetDrop

// Exec is the no-proxy session's landing; a proxied boot refuses here — the drop
// proves nothing about the netns, only `proxy exec` proves both.
func Exec(_ context.Context, argv []string) error {
	if len(argv) == 0 {
		return errors.New("no command to exec")
	}
	if proxy.Proxied() {
		return errors.New("egress wall boot: exec must land on `ccboxtools proxy exec` — exec proves the drop but not the netns")
	}
	if err := verifyAndResetDrop(); err != nil {
		return fmt.Errorf("exec session must arrive as ccbox with no caps and no_new_privs: %w", err)
	}
	return ExecArgv(argv)
}

// ExecArgv execs argv with the shell `exec`'s semantics — the process is replaced in
// place, its pid and TTY preserved.
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
