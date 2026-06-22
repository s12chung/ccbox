package docker

import (
	"context"
	"io"
	"io/fs"
	"log/slog"
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

// Proxy runs the tinyproxy egress container in the foreground (docker run --rm),
// streaming its logs until interrupted. configFS holds the tinyproxy configs, copied
// into the container at tinyproxyDir. Blocks until SIGINT/SIGTERM stops it.
func (c *Client) Proxy(ctx context.Context, configFS fs.FS) error {
	c.ensureNetwork(ctx)
	// Tear the network down on exit, but only if we're its last user. A still-running
	// devbox keeps the wall attached (expected) — leave it; `proxy clean` or the next
	// proxy run handles it. Registered before the container defer so LIFO removes the
	// egress container first.
	defer log.Defer("remove wall network", func() error {
		err := c.ProxyClean(context.Background())
		if errdefs.IsPermissionDenied(err) {
			slog.Info("keeping wall network around", "reason", err)
			return nil
		}
		return err
	})

	if err := c.ensureImage(ctx, proxyImage); err != nil {
		return err
	}
	// Clear any stale egress container so the fixed name is free.
	_ = c.cli.ContainerRemove(ctx, egressName, container.RemoveOptions{Force: true})

	resp, err := c.cli.ContainerCreate(ctx,
		&container.Config{Image: proxyImage},
		&container.HostConfig{NetworkMode: container.NetworkMode(networkName)},
		nil, nil, egressName)
	if err != nil {
		return err
	}
	id := resp.ID
	defer log.Defer("remove container", func() error {
		return c.cli.ContainerRemove(context.Background(), id, container.RemoveOptions{Force: true})
	})

	// Copy our configs in BEFORE start: the image's CMD only generates a (permissive)
	// default config when tinyproxy.conf is absent, so ours must be present first or the
	// wall starts open.
	configTar, err := embedfs.ToTar(configFS)
	if err != nil {
		return err
	}
	if err := c.cli.CopyToContainer(ctx, id, tinyproxyDir, configTar, container.CopyToContainerOptions{}); err != nil {
		return err
	}

	if err := c.cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return err
	}

	// Foreground: Ctrl-C stops the container, which ends the log stream below.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)
	go func() {
		<-sig
		timeout := 5
		_ = c.cli.ContainerStop(context.Background(), id, container.StopOptions{Timeout: &timeout})
	}()

	logs, err := c.cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true, ShowStderr: true, Follow: true,
	})
	if err != nil {
		return err
	}
	defer log.Defer("close logs", logs.Close)
	_, err = stdcopy.StdCopy(proxyColorWriter(os.Stdout), proxyColorWriter(os.Stderr), logs)
	return err
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

// ProxyClean removes the wall network (errors if it's gone or still in use).
func (c *Client) ProxyClean(ctx context.Context) error {
	return c.cli.NetworkRemove(ctx, networkName)
}
