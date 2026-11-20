// Package entrypoint is the container's fail-closed boot: bring up and prove the
// egress wall, install the env's CLI once, then idle as the supervisor — no
// workload ever runs at boot; sessions arrive by exec, proven first.
package entrypoint

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/install"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/identity"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/proxy"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/cmdgen"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/supervise"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/vnc"
)

// Run boots the container: the egress wall's boot when the proxy env is present,
// the plain boot otherwise.
func Run(ctx context.Context, vncInfo *pkginfo.VNCInfo) error {
	isProxy := proxy.Proxied()
	if err := identity.Verify(ctx, isProxy); err != nil {
		return err
	}
	var svcs []*supervise.Respawnable
	if isProxy {
		if err := identity.Harden(); err != nil {
			return err
		}
		portBindings, gatewayPorts, err := portBindingsAndGateway(vncInfo != nil)
		if err != nil {
			return err
		}
		tproxy, err := proxy.CreateNetworkAndForkChild(ctx, portBindings, gatewayPorts)
		if err != nil {
			return err
		}
		svcs = append(svcs, tproxy)
	} else if err := install.FromEnv(ctx); err != nil {
		return err
	}
	if vncInfo != nil {
		desktopSvcs, err := vnc.Serve(vncInfo)
		if err != nil {
			return err
		}
		svcs = append(svcs, desktopSvcs...)
	}
	return supervise.Run(ctx, func() { log.Info(pkginfo.ReadyMarker) }, svcs...)
}

// portBindingsAndGateway reads the run's port asks off the container env.
func portBindingsAndGateway(serveVNC bool) ([]uint16, []uint16, error) {
	portBindings, err := pkginfo.EnvPorts(pkginfo.PortBindingsTCPEnvVar)
	if err != nil {
		return nil, nil, err
	}
	if serveVNC && !slices.Contains(portBindings, pkginfo.VNCPort) {
		portBindings = append(portBindings, pkginfo.VNCPort)
	}
	gatewayPorts, err := pkginfo.EnvPorts(pkginfo.HostGatewayTCPEnvVar)
	if err != nil {
		return nil, nil, err
	}
	return portBindings, gatewayPorts, nil
}

// RunChild is the proxy child's landing: the proof, then the boot's install —
// Run marks the devbox ready.
func RunChild(ctx context.Context) error {
	if err := proxy.Validate(ctx); err != nil {
		return err
	}
	return install.FromEnv(ctx)
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
	if err := (cmdgen.SyscallCmd{Bin: argv[0], Args: argv[1:]}).Run(); err != nil {
		return fmt.Errorf("exec %s: %w", argv[0], err)
	}
	return nil
}

// ExecDesktop is the desktop session's landing inside the egress wall's netns —
// the session execs the image's XFCE on the bare server's display.
func ExecDesktop(ctx context.Context, vncInfo *pkginfo.VNCInfo) error {
	cmd, err := vnc.SetupDesktop(ctx, vncInfo)
	if err != nil {
		return err
	}
	if err := vnc.SendReady(); err != nil {
		return fmt.Errorf("vnc requested: %w", err)
	}
	if err := cmd.SyscallCmd().Run(); err != nil {
		return fmt.Errorf("vnc requested: %w", err)
	}
	return nil
}
