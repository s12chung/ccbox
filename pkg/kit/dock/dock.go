// Package dock holds generic Docker Engine operations, independent of the devbox
// lifecycle — things that can only be done from inside a container the daemon spawns.
package dock

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/prompt"
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

// EnsureOwnedVolume creates the named volume (with labels) if absent and chowns it to uid
func EnsureOwnedVolume(ctxD *CtxD, image, volumeName, uid string, labels map[string]string) error {
	switch _, err := ctxD.D.VolumeInspect(ctxD.Ctx, volumeName); {
	case err == nil:
		return nil
	case !errdefs.IsNotFound(err):
		return err
	}
	if _, err := ctxD.D.VolumeCreate(ctxD.Ctx, volume.CreateOptions{Name: volumeName, Labels: labels}); err != nil {
		return err
	}
	return ChownVolume(ctxD, image, volumeName, uid+":"+uid)
}

// ChownVolume chown's volume to owner ("uid:gid") via a throwaway root container  running image.
func ChownVolume(ctxD *CtxD, image, volumeName, owner string) error {
	mountPoint := "/mnt"

	resp, err := ctxD.D.ContainerCreate(ctxD.Ctx,
		&container.Config{
			Image:      image,
			User:       "0:0",
			Entrypoint: []string{"chown", owner, mountPoint},
		},
		&container.HostConfig{
			NetworkMode: "none",
			Binds:       []string{volumeName + ":" + mountPoint},
		},
		nil, nil, "")
	if err != nil {
		return err
	}
	defer log.Defer("remove chown container", func() error {
		return ctxD.D.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
	})

	// Register the wait before start so a fast exit isn't missed.
	statusCh, errCh := ctxD.D.ContainerWait(ctxD.Ctx, resp.ID, container.WaitConditionNextExit)
	if err := ctxD.D.ContainerStart(ctxD.Ctx, resp.ID, container.StartOptions{}); err != nil {
		return err
	}
	select {
	case err := <-errCh:
		return err
	case st := <-statusCh:
		if st.StatusCode != 0 {
			return fmt.Errorf("chown volume %q: container exited %d", volumeName, st.StatusCode)
		}
		return nil
	}
}

// EnsureImageExists pulls ref only when it isn't present locally (the SDK, unlike the
// CLI, never auto-pulls on create). Inspect resolves the digest-pinned ref that a
// reference filter would miss, so a cached image isn't re-pulled (or wrongly
// reported absent when the host is offline) each run.
func EnsureImageExists(ctxD *CtxD, ref string) error {
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

// RunInteractive wires the local terminal to the container: raw mode, a hijacked
// bidirectional stream, and SIGWINCH-driven resize. docker (the daemon) owns the
// pty; we only shuttle bytes and window sizes.
//
// Discipline: every exit path must restore the terminal, so this returns errors
// rather than calling os.Exit/log.Fatal (which skip defers). On a kill/hangup it
// stops the container instead of exiting, so this same unwind still runs.
func RunInteractive(ctxD *CtxD, id string) (int, error) {
	att, err := ctxD.D.ContainerAttach(ctxD.Ctx, id, container.AttachOptions{
		Stream: true, Stdin: true, Stdout: true, Stderr: true,
	})
	if err != nil {
		return 0, err
	}
	defer att.Close()

	if restore, ok := prompt.RawTerminal(); ok {
		defer log.Defer("restore terminal", restore)
	}

	// Register the wait before start so a fast exit isn't missed. NextExit (not
	// NotRunning) is essential: a created, not-yet-started container already satisfies
	// "not running", so NotRunning returns immediately with a bogus exit code 0.
	statusCh, errCh := ctxD.D.ContainerWait(ctxD.Ctx, id, container.WaitConditionNextExit)

	if err := ctxD.D.ContainerStart(ctxD.Ctx, id, container.StartOptions{}); err != nil {
		return 0, err
	}

	// Forward window resizes to the container's tty.
	stopResizes := prompt.ForwardResizes(func(h, w uint) {
		_ = ctxD.D.ContainerResize(ctxD.Ctx, id, container.ResizeOptions{Height: h, Width: w})
	})
	defer stopResizes()

	// On a kill/hangup, stop the container; that EOFs the output copy below, so the
	// normal unwind restores the terminal and (via Run's defer) removes the container.
	// Raw-mode Ctrl-C already goes to the container, so SIGINT here only fires for
	// non-tty stdin / `kill -INT`.
	kill := make(chan os.Signal, 1)
	signal.Notify(kill, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)
	defer signal.Stop(kill)
	go func() {
		<-kill
		timeout := 5
		_ = ctxD.D.ContainerStop(context.Background(), id, container.StopOptions{Timeout: &timeout})
	}()

	// Stdin → container (leaks on the os.Stdin read at process exit — fine for a CLI).
	go func() {
		_, _ = io.Copy(att.Conn, os.Stdin)
		_ = att.CloseWrite()
	}()
	// Container → stdout (tty merges stdout+stderr). Blocks until the container exits,
	// flushing all output before the deferred terminal restore.
	_, _ = io.Copy(os.Stdout, att.Reader)

	select {
	case err := <-errCh:
		return 0, err
	case st := <-statusCh:
		return int(st.StatusCode), nil
	}
}
