package docker

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"

	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/kit/tinyproxy"
	"github.com/s12chung/ccbox/pkg/util/flock"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/klean"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// The egress proxy image and the in-container dir its configs are served from: the
// live host dir is bound there read-only. Pinned by digest: the proxy is a security
// boundary, so it must not move on its own — bump this deliberately to pick up
// upstream patches.
const (
	proxyImage   = "kalaksi/tinyproxy@sha256:b534ce213f2c88c30aea409935ca7a0af19ddf515dd0f96196e8e07f02a5ca36"
	tinyproxyDir = "/etc/tinyproxy"
)

// ProxyOptions configures the egress wall
type ProxyOptions struct {
	HostDir    string // the live host dir, bound read-only at tinyproxyDir
	HoldersDir string // the host dir of the proxy session's holder flock entries

	BeforeStart func() error // before the proxy starts
	OnRefresh   func() error // on refreshing proxy
	OnStop      func() error // runs once the proxy is torn down

	LogFile string         // the host file the proxy's creator appends the container's logs to
	Log     *tinyproxy.Log // the optional foreground stream
}

// holdProxy joins the proxy's holder session, ensuring the egress wall is up: the first
// holder starts the proxy, a latecomer adopts the running one — or starts it back up
// when it died on its holder — and the leave tears it down only for the last holder out.
// The leave is nil on error.
func holdProxy(ctxD *dock.CtxD, o ProxyOptions) (func() error, error) {
	ensure := func() error { return ensureProxy(ctxD, o) }
	leave, err := flock.MultiFlock{Dir: o.HoldersDir}.Join(
		ensure, ensure, // verify == ensure: a latecomer heals a proxy that died on its holder
		func() error { return stopProxy(ctxD, o) },
	)
	if err != nil {
		return nil, err
	}
	return klean.SwallowErr(leave, flock.ErrNotLast), nil
}

// ensureProxy brings the egress wall up: it replaces a stopped or stale proxy — the
// takeover — and adopts a running one by refreshing its configs and reloading, so the
// holder's current configs land; then it streams the proxy's logs to LogFile, with Log
// as the optional foreground view.
func ensureProxy(ctxD *dock.CtxD, o ProxyOptions) error {
	ensureNetwork(ctxD)

	running, err := isProxyRunning(ctxD)
	if err != nil {
		return err
	}
	if !running { // the creator anchors the container's log stream: one writer per container
		if err := startProxy(ctxD, o); err != nil {
			return err
		}
		if err := streamLogFile(ctxD, o); err != nil {
			return err
		}
	} else if err := refreshProxy(ctxD, o); err != nil {
		if !errdefs.IsNotFound(err) {
			return err
		}
		return ensureProxy(ctxD, o) // the proxy died mid-adopt: a retry heals it, the creator now
	}
	return streamProxyLogs(ctxD, o) // nil Log streams nothing
}

func refreshProxy(ctxD *dock.CtxD, o ProxyOptions) error {
	if o.OnRefresh != nil {
		if err := o.OnRefresh(); err != nil {
			return err
		}
	}
	return proxyReload(ctxD)
}

// startProxy replaces any stale proxy container with a fresh one, seeded by BeforeStart
// and bound to the live dir read-only.
func startProxy(ctxD *dock.CtxD, o ProxyOptions) error {
	if err := dock.EnsureImageExists(ctxD, proxyImage); err != nil {
		return err
	}
	// Clear any stale proxy container so the fixed name is free.
	_ = ctxD.D.ContainerRemove(ctxD.Ctx, proxyContainerName, container.RemoveOptions{Force: true})

	if o.BeforeStart != nil {
		if err := o.BeforeStart(); err != nil {
			return err
		}
	}

	resp, err := ctxD.D.ContainerCreate(ctxD.Ctx,
		&container.Config{Image: proxyImage},
		&container.HostConfig{
			NetworkMode: proxyNetworkName,
			Binds:       mountSpecs([]Mount{NewBind(o.HostDir, tinyproxyDir).ReadOnly()}),
		},
		nil, nil, proxyContainerName)
	if err != nil {
		return err
	}
	if err = ctxD.D.ContainerStart(ctxD.Ctx, resp.ID, container.StartOptions{}); err != nil {
		return err
	}
	// The internal proxy network has no upstream route: the proxy forwards over
	// the engine's default bridge instead. runDevbox re-asserts the connect on
	// every run start, so an adopted proxy gets egress from it too.
	_ = ctxD.D.NetworkConnect(ctxD.Ctx, bridgeNetworkName, resp.ID, nil)
	return nil
}

// streamLogFile appends the container's log stream to the session's log file — the file
// view every holder shares.
func streamLogFile(ctxD *dock.CtxD, o ProxyOptions) error {
	f, err := os.OpenFile(o.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, ioutil.File) // #nosec G304 -- the session's own log file
	if err != nil {
		return err
	}
	return streamLogs(ctxD, &tinyproxy.Log{Writer: f}) // uncolored: it's a file
}

// streamProxyLogs tails the proxy's logs through the foreground Log; nil streams nothing.
func streamProxyLogs(ctxD *dock.CtxD, o ProxyOptions) error {
	if o.Log == nil {
		return nil
	}
	return streamLogs(ctxD, o.Log)
}

// streamLogs tails the proxy container's logs through l in a goroutine; l closes with
// the stream's reader when the stream ends with the container, or immediately on an error.
func streamLogs(ctxD *dock.CtxD, l *tinyproxy.Log) error {
	logs, err := dock.FollowLogs(ctxD, proxyContainerName)
	if err != nil {
		return errors.Join(l.Close(), err)
	}
	logDone := l.StreamGo(logs)
	go func() {
		<-logDone
		log.WarnErr("close proxy logs", errors.Join(l.Close(), logs.Close()))
	}()
	return nil
}

// stopProxy tears the egress wall down: it stops and removes the proxy container, runs
// OnStop, then removes the network. Missing pieces are already-clean, not errors.
func stopProxy(ctxD *dock.CtxD, o ProxyOptions) error {
	timeout := 5
	err := ctxD.D.ContainerStop(context.Background(), proxyContainerName, container.StopOptions{Timeout: &timeout})
	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	err = ctxD.D.ContainerRemove(context.Background(), proxyContainerName, container.RemoveOptions{Force: true})
	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	if o.OnStop != nil {
		if err := o.OnStop(); err != nil {
			return err
		}
	}
	return tearIdleNetwork(ctxD)
}

// Proxy runs the tinyproxy container in the foreground (docker run --rm),
// streaming its logs through Log until interrupted: SIGINT/SIGTERM, or Log.Stop closing —
// the proxy exits on its own. Log.Stop must be set: a stopped proxy would hang the wait.
// holdProxy holds the egress wall's session for the command — the proxy outlives it whenever
// another holder (a run, another proxy command) remains.
func Proxy(ctxD *dock.CtxD, o ProxyOptions) error {
	if o.Log == nil || o.Log.Stop == nil {
		return errors.New("Log.Stop is required: the foreground wait ends when the log stream does")
	}
	leave, err := holdProxy(ctxD, o)
	if err != nil {
		return err
	}
	defer log.Defer("proxy leave", leave)

	// Foreground: block until Ctrl-C.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)
	select {
	case <-sig:
	case <-o.Log.Stop:
	}
	return nil
}

// ensureNetwork creates the internal proxy network if absent. Like the Makefile's
// `docker network create ... || true`, a pre-existing network is not an error.
func ensureNetwork(ctxD *dock.CtxD) {
	_, _ = ctxD.D.NetworkCreate(ctxD.Ctx, proxyNetworkName, network.CreateOptions{
		Driver:   "bridge",
		Internal: true,
	})
}

// tearIdleNetwork tears the network down only if we're its last user. A still-running devbox keeps
// the proxy attached (expected) — leave it; `proxy clean` can clean it.
func tearIdleNetwork(ctxD *dock.CtxD) error {
	err := ProxyClean(ctxD)
	if errdefs.IsPermissionDenied(err) {
		log.Infof("keeping proxy network around: %v", err)
		return nil
	}
	return err
}

// proxyReload makes a running proxy re-read its configs from HostDir. Tinyproxy reloads
// on SIGUSR1 — never SIGHUP: with the image's foreground `-d`, SIGHUP is unhandled and
// would kill the proxy. ContainerKill is Docker's generic "send a signal" API — it only
// kills because the default signal is SIGKILL.
func proxyReload(ctxD *dock.CtxD) error {
	return ctxD.D.ContainerKill(ctxD.Ctx, proxyContainerName, "USR1")
}

// ProxyClean removes the proxy network. A missing network is already clean (not an error);
// an in-use one still errors.
func ProxyClean(ctxD *dock.CtxD) error {
	if err := ctxD.D.NetworkRemove(ctxD.Ctx, proxyNetworkName); err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	return nil
}

// isProxyRunning reports whether the proxy container is up. A missing or stopped proxy is
// false, not an error.
func isProxyRunning(ctxD *dock.CtxD) (bool, error) {
	info, err := ctxD.D.ContainerInspect(ctxD.Ctx, proxyContainerName)
	if errdefs.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.State.Running, nil
}
