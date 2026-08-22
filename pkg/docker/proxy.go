package docker

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/util/cleanup"
	"github.com/s12chung/ccbox/pkg/util/embedfs"
	"github.com/s12chung/ccbox/pkg/util/log"
	"github.com/s12chung/ccbox/pkg/util/prompt"
)

// The egress proxy image and the in-container dir its configs are copied into.
// Pinned by digest: the wall is a security boundary, so it must not move on its
// own — bump this deliberately to pick up upstream patches.
const (
	proxyImage   = "kalaksi/tinyproxy@sha256:b534ce213f2c88c30aea409935ca7a0af19ddf515dd0f96196e8e07f02a5ca36"
	tinyproxyDir = "/etc/tinyproxy"
)

// ProxyOptions configures the egress wall: Config holds the tinyproxy configs copied
// into tinyproxyDir, and Overrides seeds files (the generated allow.txt) into them.
type ProxyOptions struct {
	Config    fs.FS
	Overrides map[string][]byte
}

// Proxy runs the tinyproxy egress container in the foreground (docker run --rm),
// streaming its logs until interrupted. Blocks until SIGINT/SIGTERM stops it.
func Proxy(ctxD *dock.CtxD, o ProxyOptions) error {
	// Wall exits on its own (logFn closes stop).
	stop := make(chan struct{})
	cleanup, err := proxyStart(ctxD, o, func(logs io.ReadCloser) error {
		_, err := stdcopy.StdCopy(proxyColorWriter(os.Stdout), proxyColorWriter(os.Stderr), logs)
		close(stop)
		return err
	})
	if err != nil {
		return err
	}

	// Foreground: block until Ctrl-C.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)
	select {
	case <-sig:
	case <-stop:
	}
	return cleanup()
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

// ensureImage pulls ref only when it isn't present locally (the SDK, unlike the
// CLI, never auto-pulls on create). Inspect resolves the digest-pinned ref that a
// reference filter would miss, so a cached image isn't re-pulled (or wrongly
// reported absent when the host is offline) each run.
func ensureImage(ctxD *dock.CtxD, ref string) error {
	if _, err := ctxD.D.ImageInspect(ctxD.Ctx, ref); err == nil {
		return nil
	}
	readCloser, err := ctxD.D.ImagePull(ctxD.Ctx, ref, image.PullOptions{})
	if err != nil {
		return err
	}
	defer log.Defer("close image pull", readCloser.Close)
	return prompt.DisplayProgress(readCloser)
}

// ensureNetwork creates the internal wall network if absent. Like the Makefile's
// `docker network create ... || true`, a pre-existing network is not an error.
func ensureNetwork(ctxD *dock.CtxD) {
	_, _ = ctxD.D.NetworkCreate(ctxD.Ctx, networkName, network.CreateOptions{
		Driver:   "bridge",
		Internal: true,
	})
}

// ProxyClean removes the wall network. A missing network is already clean (not an error);
// an in-use one still errors.
func ProxyClean(ctxD *dock.CtxD) error {
	if err := ctxD.D.NetworkRemove(ctxD.Ctx, networkName); err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	return nil
}

// proxyStart brings the egress wall up and streams its logs via logFn in a goroutine.
func proxyStart(ctxD *dock.CtxD, o ProxyOptions, logFn func(logs io.ReadCloser) error) (teardown func() error, err error) {
	var stack cleanup.Stack
	// Unwind partial setup if we bail before returning teardown.
	defer func() {
		if err != nil {
			stack.Run()
		}
	}()

	ensureNetwork(ctxD)
	// Tear the network down only if we're its last user. A still-running devbox keeps
	// the wall attached (expected) — leave it; `proxy clean` can clean it.
	stack.Push("remove wall network", func() error {
		err := ProxyClean(ctxD)
		if errdefs.IsPermissionDenied(err) {
			log.Infof("keeping wall network around: %v", err)
			return nil
		}
		return err
	})

	if err = ensureImage(ctxD, proxyImage); err != nil {
		return nil, err
	}
	// Clear any stale egress container so the fixed name is free.
	_ = ctxD.D.ContainerRemove(ctxD.Ctx, egressName, container.RemoveOptions{Force: true})

	resp, err := ctxD.D.ContainerCreate(ctxD.Ctx,
		&container.Config{Image: proxyImage},
		&container.HostConfig{NetworkMode: networkName},
		nil, nil, egressName)
	if err != nil {
		return nil, err
	}
	id := resp.ID
	stack.Push("remove container", func() error {
		return ctxD.D.ContainerRemove(context.Background(), id, container.RemoveOptions{Force: true})
	})

	configTar, err := embedfs.ToTar(o.Config, o.Overrides)
	if err != nil {
		return nil, err
	}
	if err = ctxD.D.CopyToContainer(ctxD.Ctx, id, tinyproxyDir, configTar, container.CopyToContainerOptions{}); err != nil {
		return nil, err
	}
	if err = ctxD.D.ContainerStart(ctxD.Ctx, id, container.StartOptions{}); err != nil {
		return nil, err
	}
	stack.Push("container stop", func() error {
		timeout := 5
		return ctxD.D.ContainerStop(context.Background(), id, container.StopOptions{Timeout: &timeout})
	})

	logs, err := ctxD.D.ContainerLogs(ctxD.Ctx, id, container.LogsOptions{
		ShowStdout: true, ShowStderr: true, Follow: true,
	})
	if err != nil {
		return nil, err
	}
	stack.Push("close logs", logs.Close)

	logDone := make(chan error)
	go func() {
		logErr := logFn(logs)
		if errors.Is(logErr, net.ErrClosed) { // cleanup closed the stream; not a real failure
			logErr = nil
		}
		logDone <- logErr
		close(logDone)
	}()

	return func() error {
		stack.Run()
		return <-logDone
	}, nil
}
