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

	"github.com/s12chung/ccbox/pkg/embedfs"
	"github.com/s12chung/ccbox/pkg/log"
	"github.com/s12chung/ccbox/pkg/prompt"
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
func (c *Client) Proxy(ctx context.Context, o ProxyOptions) error {
	// Wall exits on its own (logFn closes stop).
	stop := make(chan struct{})
	logDone, err := c.proxyWrap(ctx, o, func() error {
		// Foreground: block until Ctrl-C
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sig)

		select {
		case <-sig:
		case <-stop:
		}
		return nil
	}, func(logs io.ReadCloser) error {
		_, err := stdcopy.StdCopy(proxyColorWriter(os.Stdout), proxyColorWriter(os.Stderr), logs)
		close(stop)
		return err
	})
	var logErr error
	if logDone != nil {
		logErr = <-logDone
	}
	return errors.Join(err, logErr)
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
func (c *Client) ensureImage(ctx context.Context, ref string) error {
	if _, err := c.cli.ImageInspect(ctx, ref); err == nil {
		return nil
	}
	readCloser, err := c.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return err
	}
	defer log.Defer("close image pull", readCloser.Close)
	return prompt.DisplayProgress(readCloser)
}

// ensureNetwork creates the internal wall network if absent. Like the Makefile's
// `docker network create ... || true`, a pre-existing network is not an error.
func (c *Client) ensureNetwork(ctx context.Context) {
	_, _ = c.cli.NetworkCreate(ctx, networkName, network.CreateOptions{
		Driver:   "bridge",
		Internal: true,
	})
}

// ProxyClean removes the wall network. A missing network is already clean (not an error);
// an in-use one still errors.
func (c *Client) ProxyClean(ctx context.Context) error {
	if err := c.cli.NetworkRemove(ctx, networkName); err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	return nil
}

// proxyWrap starts an egress wall container, runs fn against it, then cleans it up.
// logFn streams the wall's logs in parallel; logDone delivers its result once it finishes.
func (c *Client) proxyWrap(ctx context.Context, o ProxyOptions, fn func() error, logFn func(logs io.ReadCloser) error) (logDone chan error, err error) {
	c.ensureNetwork(ctx)
	// Tear the network down on exit, but only if we're its last user. A still-running
	// devbox keeps the wall attached (expected) — leave it; `proxy clean` can clean it
	defer log.Defer("remove wall network", func() error {
		err := c.ProxyClean(context.Background())
		if errdefs.IsPermissionDenied(err) {
			log.Infof("keeping wall network around: %v", err)
			return nil
		}
		return err
	})

	if err = c.ensureImage(ctx, proxyImage); err != nil {
		return nil, err
	}
	// Clear any stale egress container so the fixed name is free.
	_ = c.cli.ContainerRemove(ctx, egressName, container.RemoveOptions{Force: true})

	resp, err := c.cli.ContainerCreate(ctx,
		&container.Config{Image: proxyImage},
		&container.HostConfig{NetworkMode: container.NetworkMode(networkName)},
		nil, nil, egressName)
	if err != nil {
		return nil, err
	}
	id := resp.ID
	defer log.Defer("remove container", func() error {
		return c.cli.ContainerRemove(context.Background(), id, container.RemoveOptions{Force: true})
	})

	configTar, err := embedfs.ToTar(o.Config, o.Overrides)
	if err != nil {
		return nil, err
	}
	if err = c.cli.CopyToContainer(ctx, id, tinyproxyDir, configTar, container.CopyToContainerOptions{}); err != nil {
		return nil, err
	}
	if err = c.cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return nil, err
	}
	defer log.Defer("container stop", func() error {
		timeout := 5
		return c.cli.ContainerStop(context.Background(), id, container.StopOptions{Timeout: &timeout})
	})

	logs, err := c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true, ShowStderr: true, Follow: true,
	})
	if err != nil {
		return nil, err
	}
	defer log.Defer("close logs", logs.Close)

	logDone = make(chan error)
	go func() {
		logErr := logFn(logs)
		if errors.Is(logErr, net.ErrClosed) { // cleanup closed the stream; not a real failure
			logErr = nil
		}
		logDone <- logErr
		close(logDone)
	}()
	return logDone, fn()
}
