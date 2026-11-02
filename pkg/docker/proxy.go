package docker

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"

	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/kit/tinyproxy"
	"github.com/s12chung/ccbox/pkg/util/errs"
	"github.com/s12chung/ccbox/pkg/util/klean"
	"github.com/s12chung/ccbox/pkg/util/term"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/flock"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// ProxyOptions configures the egress wall on a boot
type ProxyOptions struct {
	HostDir string // the live host dir, bound read-only at pkginfo.ProxyMount

	BeforeStart func() error // before the proxy starts
	OnRefresh   func() error // on refreshing proxy
	OnStop      func() error // runs once the proxy is torn down

	LogFile string // the host file the proxy's own log lines append to
}

// holdDevbox joins the devbox's holder session, ensuring the boot is up
func holdDevbox(ctxD *dock.CtxD, o RunOptions) (_ func() error, err error) {
	leave, err := flock.MultiFlock{Dir: o.HoldersDir}.Join(
		func() error { return verifyAndHeal(ctxD, o) },
		func() error { return createDevbox(ctxD, o) },
		func() error { return stopDevbox(ctxD, o) },
	)
	if err != nil {
		return nil, err
	}

	leave = klean.SwallowErr(leave, flock.ErrNotLast)
	defer func() { err = errs.JoinOnErr(err, leave) }()

	if o.Proxy != nil {
		if err = dock.AwaitLogMarker(ctxD, o.Name, pkginfo.ReadyMarker); err != nil {
			return nil, err
		}
		if err = o.Proxy.OnRefresh(); err != nil {
			return nil, err
		}
	}
	if err = streamLogs(ctxD, o.Name, o.Foreground); err != nil {
		return nil, err
	}
	return leave, nil
}

func verifyAndHeal(ctxD *dock.CtxD, o RunOptions) error {
	own, err := claimDevbox(ctxD, o)
	if err != nil {
		return err
	}
	if own == nil {
		return createDevbox(ctxD, o)
	}
	return nil
}

// claimDevbox returns the boot's own running devbox, nil when none is running —
// one running devbox per variant
func claimDevbox(ctxD *dock.CtxD, o RunOptions) (*container.InspectResponse, error) {
	own, err := dock.InspectRunning(ctxD, o.Name)
	if err = errs.Swallow(err, dock.ErrNotRunning); err != nil {
		return nil, err
	}
	if own != nil && !o.matchesContainer(own.Config.Labels) {
		return nil, conflictErr(o.Name, own.Config.Labels)
	}
	return own, nil
}

// DevboxRunning reports whether the boot's own devbox is running
func DevboxRunning(ctxD *dock.CtxD, o RunOptions) (bool, error) {
	own, err := claimDevbox(ctxD, o)
	return own != nil, err
}

// conflictErr refuses the boot: a devbox under the boot's own name is already running
// with diverging labels — the error names it and how to clear it
func conflictErr(name string, labels map[string]string) error {
	proxy := "without the proxy"
	if labels[ccboxProjectLabelKey] == "" {
		proxy = "of an older ccbox, its proxy state unknown"
	} else if labels[ccboxHasProxyLabelKey] == trueVolValue {
		proxy = "with the proxy"
	}
	return fmt.Errorf("devbox %s is already running for this project, %s — exit its session or `docker rm -f %s`",
		name, proxy, name)
}

// createDevbox starts the devbox fresh, streaming its logs to the run's views;
// a failed create tears the half-boot down, leaving nothing behind
func createDevbox(ctxD *dock.CtxD, o RunOptions) (err error) {
	defer func() {
		err = errs.JoinOnErr(err, func() error { return stopDevbox(ctxD, o) })
	}()

	if err = startDevbox(ctxD, o); err != nil {
		return err
	}
	opts, err := o.streamOptions()
	if err != nil {
		return err
	}
	return dock.StreamLogs(ctxD, o.Name, opts)
}

// startDevbox replaces any stale devbox with a fresh one off the run's options
func startDevbox(ctxD *dock.CtxD, o RunOptions) error {
	if err := dock.EnsureImageExists(ctxD, o.Tag); err != nil {
		return err
	}
	// Clear any stale devbox container so the fixed name is free.
	_ = ctxD.D.ContainerRemove(ctxD.Ctx, o.Name, container.RemoveOptions{Force: true})

	if o.Proxy != nil {
		if err := o.Proxy.BeforeStart(); err != nil {
			return err
		}
	}

	hostConfig, err := runHostConfig(ctxD, o)
	if err != nil {
		return err
	}
	resp, err := ctxD.D.ContainerCreate(ctxD.Ctx, runConfig(o), hostConfig, nil, nil, o.Name)
	if err != nil {
		return err
	}
	return ctxD.D.ContainerStart(ctxD.Ctx, resp.ID, container.StartOptions{})
}

// streamLogs tails the container's logs through fg in a goroutine, closing
// its Stop with the stream. A nil fg streams nothing.
func streamLogs(ctxD *dock.CtxD, name string, fg *Foreground) error {
	if fg == nil {
		return nil
	}
	logs, err := dock.FollowLogs(ctxD, name)
	if err != nil {
		return err
	}
	w := term.NewColorWriter(fg.Writer, tinyproxy.LevelColor)
	go func() {
		_, copyErr := io.Copy(w, logs)
		close(fg.Stop)
		log.WarnErr("close devbox logs", errors.Join(
			errs.Swallow(copyErr, io.ErrClosedPipe, os.ErrClosed, net.ErrClosed), logs.Close()))
	}()
	return nil
}

// stopDevbox tears the devbox down detached, so a cancelled caller's ctx can't block
// the cleanups; already-clean pieces are no error
func stopDevbox(ctxD *dock.CtxD, o RunOptions) error {
	ctxD = ctxD.BackgroundedCtx()
	timeout := 5
	err := ctxD.D.ContainerStop(ctxD.Ctx, o.Name, container.StopOptions{Timeout: &timeout})
	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	err = ctxD.D.ContainerRemove(ctxD.Ctx, o.Name, container.RemoveOptions{Force: true})
	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	if o.Proxy != nil {
		return o.Proxy.OnStop()
	}
	return nil
}

// Run boots the devbox detached, streaming its logs to the foreground until interrupted
func Run(ctxD *dock.CtxD, o RunOptions) error {
	if o.Foreground == nil {
		return errors.New("the run needs a Foreground view: it streams the boot's logs")
	}
	o.Foreground.Stop = make(chan struct{}) // the wait ends when the boot's log stream does
	leave, err := holdDevbox(ctxD, o)
	if err != nil {
		return err
	}
	defer log.Defer("devbox leave", leave)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)
	select {
	case <-sig:
	case <-o.Foreground.Stop: // the stream ends with the container
	}
	return nil
}

// Exec runs the boot's exec session in the devbox, wired to the local terminal
func Exec(ctxD *dock.CtxD, o RunOptions) error {
	if o.Exec == nil {
		return errors.New("Exec needs the session's exec options")
	}
	leave, err := holdDevbox(ctxD, o)
	if err != nil {
		return err
	}
	defer log.Defer("devbox leave", leave)
	return dock.ExecInteractive(ctxD, o.Name, o.Exec.Cmd)
}
