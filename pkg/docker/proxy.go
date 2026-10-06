package docker

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/s12chung/ccbox/ccboxtools/pkg/util/log"
	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/util/klean"
	"github.com/s12chung/ccbox/pkg/util/prompt"
)

// The egress proxy image and the in-container dir its configs are served from: the
// live host dir is bound there read-only. Pinned by digest: the proxy is a security
// boundary, so it must not move on its own — bump this deliberately to pick up
// upstream patches.
const (
	proxyImage   = "kalaksi/tinyproxy@sha256:b534ce213f2c88c30aea409935ca7a0af19ddf515dd0f96196e8e07f02a5ca36"
	tinyproxyDir = "/etc/tinyproxy"
)

// ProxyOptions configures the egress wall: BeforeStart seeds HostDir — bound read-only
// at tinyproxyDir — just before the proxy starts, OnStop runs once the proxy is torn down,
// and Log streams the proxy's logs
type ProxyOptions struct {
	HostDir     string
	BeforeStart func() error
	OnStop      func() error
	Log         *ProxyLog
}

// ProxyLog streams the proxy's logs through Writer; Colored tints each line per tinyproxy
// level, and Stop closes when the stream ends — the foreground proxy exits on its own
type ProxyLog struct {
	Writer  io.Writer
	Colored bool
	Stop    chan struct{}
}

// Log streams logs through Writer
func (pl ProxyLog) Log(logs io.ReadCloser) error {
	w := pl.Writer
	if pl.Colored {
		w = proxyColorWriter(w)
	}
	_, err := stdcopy.StdCopy(w, w, logs)
	if pl.Stop != nil {
		close(pl.Stop)
	}
	return err
}

// proxyStart brings the egress wall up and streams its logs via Log in a goroutine.
func proxyStart(ctxD *dock.CtxD, o ProxyOptions) (func() error, error) {
	var joiner klean.Joiner

	ensureNetwork(ctxD)
	joiner.Push("remove proxy network", func() error {
		return tearIdleNetwork(ctxD)
	})

	if err := dock.EnsureImageExists(ctxD, proxyImage); err != nil {
		return joiner.Run, err
	}
	// Clear any stale egress container so the fixed name is free.
	_ = ctxD.D.ContainerRemove(ctxD.Ctx, egressName, container.RemoveOptions{Force: true})

	if o.BeforeStart != nil {
		if err := o.BeforeStart(); err != nil {
			return joiner.Run, err
		}
	}

	resp, err := ctxD.D.ContainerCreate(ctxD.Ctx,
		&container.Config{Image: proxyImage},
		&container.HostConfig{
			NetworkMode: networkName,
			Binds:       mountSpecs([]Mount{NewBind(o.HostDir, tinyproxyDir).ReadOnly()}),
		},
		nil, nil, egressName)
	if err != nil {
		return joiner.Run, err
	}
	id := resp.ID
	joiner.Push("remove container", func() error {
		return ctxD.D.ContainerRemove(context.Background(), id, container.RemoveOptions{Force: true})
	})

	if err = ctxD.D.ContainerStart(ctxD.Ctx, id, container.StartOptions{}); err != nil {
		return joiner.Run, err
	}
	joiner.Push("container stop", func() error {
		timeout := 5
		return ctxD.D.ContainerStop(context.Background(), id, container.StopOptions{Timeout: &timeout})
	})

	logs, err := ctxD.D.ContainerLogs(ctxD.Ctx, id, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return joiner.Run, err
	}
	joiner.Push("close logs", logs.Close)

	if o.OnStop != nil {
		joiner.Push("on stop", o.OnStop)
	}
	logDone := streamLogs(o.Log, logs)
	return func() error {
		return errors.Join(joiner.Run(), <-logDone)
	}, nil
}

// Proxy runs the tinyproxy egress container in the foreground (docker run --rm),
// streaming its logs through Log until interrupted: SIGINT/SIGTERM, or Log.Stop closing —
// the proxy exits on its own. Log.Stop must be set: a stopped proxy would hang the wait.
func Proxy(ctxD *dock.CtxD, o ProxyOptions) error {
	if o.Log == nil || o.Log.Stop == nil {
		return errors.New("Log.Stop is required: the foreground wait ends when the log stream does")
	}
	clean, err := proxyStart(ctxD, o)
	defer log.Defer("proxy clean", clean)
	if err != nil {
		return err
	}

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

func proxyColorWriter(w io.Writer) io.Writer { return prompt.NewColorWriter(w, tinyproxyLevelColor) }

var tinyproxyLevelColors = map[string]prompt.Color{
	"CRITICAL": prompt.ColorBoldRed,
	"ERROR":    prompt.ColorRed,
	"WARNING":  prompt.ColorYellow,
	"NOTICE":   prompt.ColorGreen,
	"CONNECT":  prompt.ColorCyan,
	"INFO":     prompt.ColorDim,
}

func tinyproxyLevelColor(line string) prompt.Color {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	return tinyproxyLevelColors[fields[0]]
}

// ensureNetwork creates the internal proxy network if absent. Like the Makefile's
// `docker network create ... || true`, a pre-existing network is not an error.
func ensureNetwork(ctxD *dock.CtxD) {
	_, _ = ctxD.D.NetworkCreate(ctxD.Ctx, networkName, network.CreateOptions{
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

// streamLogs streams logs through l in a goroutine and returns a channel yielding
// its error when the stream ends.
func streamLogs(l *ProxyLog, logs io.ReadCloser) chan error {
	logDone := make(chan error)
	go func() {
		logErr := l.Log(logs)
		if errors.Is(logErr, net.ErrClosed) { // cleanup closed the stream; not a real failure
			logErr = nil
		}
		logDone <- logErr
		close(logDone)
	}()
	return logDone
}

// ProxyReload makes a running proxy re-read its configs from HostDir. Tinyproxy reloads
// on SIGUSR1 — never SIGHUP: with the image's foreground `-d`, SIGHUP is unhandled and
// would kill the proxy.
func ProxyReload(ctxD *dock.CtxD) error {
	return ctxD.D.ContainerKill(ctxD.Ctx, egressName, "USR1")
}

// ProxyClean removes the proxy network. A missing network is already clean (not an error);
// an in-use one still errors.
func ProxyClean(ctxD *dock.CtxD) error {
	if err := ctxD.D.NetworkRemove(ctxD.Ctx, networkName); err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	return nil
}

// isProxyRunning reports whether the egress container is up. A missing or stopped proxy is
// false, not an error.
func isProxyRunning(ctxD *dock.CtxD) (bool, error) {
	info, err := ctxD.D.ContainerInspect(ctxD.Ctx, egressName)
	if errdefs.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.State.Running, nil
}
