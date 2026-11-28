// Package dock holds generic Docker Engine operations, independent of the devbox lifecycle.
package dock

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"slices"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/term"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// CtxD pairs a D Engine client with the context its calls run under.
type CtxD struct {
	//nolint:containedctx // pairs the engine client with its call context by design
	Ctx context.Context
	D   *client.Client
}

// NewCtxD builds an Engine client from the environment (DOCKER_HOST etc.) under ctx.
func NewCtxD(ctx context.Context) (*CtxD, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &CtxD{Ctx: ctx, D: cli}, nil
}

// MustNewCtxD is NewCtxD, panicking on error.
func MustNewCtxD(ctx context.Context) *CtxD { return must.Get(NewCtxD(ctx)) }

// BackgroundedCtx returns a copy of ctxD whose calls outlive the caller's cancel
func (ctxD *CtxD) BackgroundedCtx() *CtxD {
	return &CtxD{Ctx: context.Background(), D: ctxD.D}
}

// EnsureImageExists pulls ref only when it isn't present locally — the SDK never auto-pulls
// on create. Inspect, not a list filter: it resolves digest-pinned refs too, so a cached
// image isn't re-pulled each run.
func EnsureImageExists(ctxD *CtxD, ref string) error {
	if _, err := ctxD.D.ImageInspect(ctxD.Ctx, ref); err == nil {
		return nil
	}
	readCloser, err := ctxD.D.ImagePull(ctxD.Ctx, ref, image.PullOptions{})
	if err != nil {
		return err
	}

	defer log.Defer("close image pull", readCloser.Close)
	return term.DisplayProgress(readCloser)
}

// ErrNotRunning is InspectRunning's sentinel for an absent or stopped container
var ErrNotRunning = errors.New("dock: container not running")

// InspectRunning returns the named running container's inspect, ErrNotRunning otherwise.
func InspectRunning(ctxD *CtxD, name string) (*container.InspectResponse, error) {
	info, err := ctxD.D.ContainerInspect(ctxD.Ctx, name)
	if errdefs.IsNotFound(err) {
		return nil, ErrNotRunning
	}
	if err != nil {
		return nil, err
	}
	if !info.State.Running {
		return nil, ErrNotRunning
	}
	return &info, nil
}

// RefuseHeldPorts errors for each published host port something already
// listens on — naming the running container publishing it, else the port
// alone — so the caller refuses before the daemon's bind error names nobody
func RefuseHeldPorts(ctxD *CtxD, bindings nat.PortMap) error {
	var errs []error
	for _, port := range slices.Sorted(maps.Keys(bindings)) {
		for _, b := range bindings[port] {
			if err := refuseHeldBinding(ctxD, b); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// refuseHeldBinding errors when something already listens on the binding's host port
func refuseHeldBinding(ctxD *CtxD, b nat.PortBinding) error {
	hostIP := b.HostIP
	if hostIP == "" {
		hostIP = "127.0.0.1" // the loopback probe answers a 0.0.0.0 bind too
	}
	addr := net.JoinHostPort(hostIP, b.HostPort)
	conn, err := (&net.Dialer{Timeout: 250 * time.Millisecond}).DialContext(ctxD.Ctx, "tcp", addr)
	taken := err == nil
	if taken {
		log.WarnErr("close the port probe", conn.Close())
	}
	if !taken {
		return nil // nothing listens
	}

	resp, err := ctxD.D.ContainerList(ctxD.Ctx, container.ListOptions{Filters: filters.NewArgs(
		filters.Arg("publish", b.HostPort),
	)})
	if err != nil {
		return fmt.Errorf("host tcp port %s is already taken", addr)
	}
	for _, c := range resp {
		if c.State != "running" || len(c.Names) == 0 {
			continue // a stopped devbox's spec still matches, but its bind is gone
		}
		holder := strings.TrimPrefix(c.Names[0], "/")
		return fmt.Errorf("host tcp port %s is held by %s — exit its session or `docker rm -f %s`", addr, holder, holder)
	}
	return fmt.Errorf("host tcp port %s is already taken", addr)
}
