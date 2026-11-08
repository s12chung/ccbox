// Package dock holds generic Docker Engine operations, independent of the devbox
// lifecycle — things that can only be done from inside a container the daemon spawns.
package dock

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"github.com/s12chung/ccbox/pkg/util/errs"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/slicex"
	"github.com/s12chung/ccbox/pkg/util/stream"
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

// OwnedVolume is a volume EnsureOwnedVolumes ensures: created (with labels) if absent.
type OwnedVolume struct {
	Name   string
	Labels map[string]string
}

// EnsureOwnedVolumes creates each missing volume (with labels) and chowns the fresh
// ones' roots to uid via ONE throwaway root container running image. Existing volumes
// are never re-chowned
func EnsureOwnedVolumes(ctxD *CtxD, image, uid string, vols []OwnedVolume) error {
	var fresh []OwnedVolume
	for _, vol := range vols {
		isFresh, err := ensureVolume(ctxD, vol)
		if err != nil {
			return err
		}
		if isFresh {
			fresh = append(fresh, vol)
		}
	}
	if len(fresh) == 0 {
		return nil
	}
	return runChownContainer(ctxD, image, uid, fresh)
}

// ensureVolume creates vol (with labels) if absent, reporting whether it was fresh.
func ensureVolume(ctxD *CtxD, vol OwnedVolume) (bool, error) {
	switch _, err := ctxD.D.VolumeInspect(ctxD.Ctx, vol.Name); {
	case err == nil:
		return false, nil
	case !errdefs.IsNotFound(err):
		return false, err
	}
	if _, err := ctxD.D.VolumeCreate(ctxD.Ctx, volume.CreateOptions{Name: vol.Name, Labels: vol.Labels}); err != nil {
		return false, err
	}
	return true, nil
}

// runChownContainer chowns the fresh volumes' roots via the throwaway root container,
// waiting for it to exit.
func runChownContainer(ctxD *CtxD, image, uid string, fresh []OwnedVolume) error {
	resp, err := ctxD.D.ContainerCreate(ctxD.Ctx,
		&container.Config{
			Image:      image,
			User:       "0:0",
			Entrypoint: append([]string{"chown", uid + ":" + uid}, slicex.Map(fresh, chownMnt)...),
		},
		&container.HostConfig{
			NetworkMode: "none",
			Binds:       slicex.Map(fresh, chownBind),
		},
		nil, nil, "")
	if err != nil {
		return err
	}
	// the chown container's remove must complete even on a cancelled ctx
	clean := ctxD.BackgroundedCtx()
	defer log.Defer("remove chown container", func() error {
		return clean.D.ContainerRemove(clean.Ctx, resp.ID, container.RemoveOptions{Force: true})
	})

	var logs bytes.Buffer
	if err := RunOnce(ctxD, resp.ID, &logs); err != nil {
		return fmt.Errorf("chown volumes %v: %w\n%s",
			slicex.Map(fresh, func(vol OwnedVolume) string { return vol.Name }), err, logs.String())
	}
	return nil
}

// chownMnt is vol's mount point in the chown container: /mnt/<volume-name>. Volume names
// are DNS-label-safe ([a-zA-Z0-9_.-]), so they're valid path segments.
func chownMnt(vol OwnedVolume) string { return "/mnt/" + vol.Name }

// chownBind is vol's bind spec in the chown container: <name>:<mount point>
func chownBind(vol OwnedVolume) string { return vol.Name + ":" + chownMnt(vol) }

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
		return nil // refused: nothing listens
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

// FollowLogs opens the container's combined follow stream as plain bytes. The daemon holds it
// open until the container exits and closes it then, so the stream's end doubles as the wait.
func FollowLogs(ctxD *CtxD, id string) (io.ReadCloser, error) {
	logs, err := ctxD.D.ContainerLogs(ctxD.Ctx, id, container.LogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		return nil, err
	}
	return &demuxReader{r: logs}, nil
}

// demuxReader reads the daemon's log stream as plain bytes, converted from the container's
// format interweaving stdout/stderr
type demuxReader struct {
	r     io.ReadCloser
	head  [8]byte
	frame []byte // the current frame's bytes not yet read out
}

func (r *demuxReader) Read(p []byte) (int, error) {
	for len(r.frame) == 0 {
		// each frame: a stdout/stderr type byte, padding, then the payload's big-endian length
		if _, err := io.ReadFull(r.r, r.head[:]); err != nil {
			return 0, err
		}
		body := make([]byte, binary.BigEndian.Uint32(r.head[4:]))
		if _, err := io.ReadFull(r.r, body); err != nil {
			return 0, err
		}
		r.frame = body
	}
	n := copy(p, r.frame)
	r.frame = r.frame[n:]
	return n, nil
}

func (r *demuxReader) Close() error { return r.r.Close() }

// RunOnce runs a created container to exit, copying its combined follow stream into w —
// the copy doubles as the wait — and fails on a non-zero exit. Callers report their own output.
func RunOnce(ctxD *CtxD, id string, w io.Writer) error {
	if err := ctxD.D.ContainerStart(ctxD.Ctx, id, container.StartOptions{}); err != nil {
		return err
	}
	logs, err := FollowLogs(ctxD, id)
	if err != nil {
		return err
	}
	defer log.Defer("close logs", logs.Close)
	if _, err := io.Copy(w, logs); err != nil {
		return err
	}
	info, err := ctxD.D.ContainerInspect(ctxD.Ctx, id)
	if err != nil {
		return err
	}
	if code := info.State.ExitCode; code != 0 {
		return fmt.Errorf("container exited %d", code)
	}
	return nil
}

// ExecInteractive wires the local terminal to a running container's exec, returning its
// exit code as errs.ExitError. A kill/hangup ends the attach, never the container.
func ExecInteractive(ctxD *CtxD, id string, cmd []string) error {
	resp, err := ctxD.D.ContainerExecCreate(ctxD.Ctx, id, container.ExecOptions{
		Cmd:          cmd,
		Tty:          true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return err
	}
	att, err := ctxD.D.ContainerExecAttach(ctxD.Ctx, resp.ID, container.ExecStartOptions{Tty: true})
	if err != nil {
		return err
	}
	defer att.Close()

	if restore, ok := term.Raw(); ok {
		defer log.Defer("restore terminal", restore)
	}

	// Forward window resizes to the exec's tty; the first sizes the pty to the terminal,
	// not the daemon's default.
	stopResizes := term.ForwardResizes(func(h, w uint) {
		_ = ctxD.D.ContainerExecResize(ctxD.Ctx, resp.ID, container.ResizeOptions{Height: h, Width: w})
	})
	defer stopResizes()

	// Stdin → exec (leaks on the os.Stdin read at process exit — fine for a CLI).
	go func() {
		_, _ = io.Copy(att.Conn, os.Stdin)
		_ = att.CloseWrite()
	}()
	// Exec → stdout (tty merges stdout+stderr); blocks until the exec exits
	_, _ = io.Copy(os.Stdout, att.Reader)

	info, err := ctxD.D.ContainerExecInspect(ctxD.Ctx, resp.ID)
	if err != nil {
		return err
	}
	return errs.Exit(info.ExitCode)
}

// CopyFileFrom copies the container's file at path into memory. CopyFrom tars single
// files under their basename, so the stream holds the one entry to read out.
func CopyFileFrom(ctxD *CtxD, id, path string) ([]byte, error) {
	stream, _, err := ctxD.D.CopyFromContainer(ctxD.Ctx, id, path)
	if err != nil {
		return nil, err
	}
	defer log.Defer("close copy stream", stream.Close)
	tr := tar.NewReader(stream)
	if _, err := tr.Next(); err != nil {
		return nil, err
	}
	return io.ReadAll(tr)
}

// CopyDirFrom copies the container's directory at dir into memory, its files keyed
// by their paths relative to dir. CopyFrom tars the tree under dir's basename.
func CopyDirFrom(ctxD *CtxD, id, dir string) (map[string][]byte, error) {
	stream, _, err := ctxD.D.CopyFromContainer(ctxD.Ctx, id, dir)
	if err != nil {
		return nil, err
	}
	defer log.Defer("close copy stream", stream.Close)
	return filesFromTar(stream, filepath.Base(dir)+"/")
}

// filesFromTar reads the regular files out of a CopyFrom stream, dropping the entries'
// name prefix — the tarred tree's basename.
func filesFromTar(stream io.Reader, prefix string) (map[string][]byte, error) {
	files := map[string][]byte{}
	tr := tar.NewReader(stream)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		files[strings.TrimPrefix(hdr.Name, prefix)] = body
	}
}

// AwaitLogMarker follows the container's logs until a line equals marker, failing closed with the
// last logs. The follow replays them, so a boot already past the marker answers at once.
func AwaitLogMarker(ctxD *CtxD, id, marker string) error {
	// a derived cancel stops the log stream at the marker, not the container's exit
	ctx, cancel := context.WithCancel(ctxD.Ctx)
	defer cancel()
	logs, err := FollowLogs(&CtxD{Ctx: ctx, D: ctxD.D}, id)
	if err != nil {
		return err
	}
	return stream.AwaitMarker(logs, marker, cancel)
}

// StreamLogs tees the whole followed log stream to the options' views — a set marker waits on
// those same teed bytes, failing closed with the last logs when the stream ends first
func StreamLogs(ctxD *CtxD, id string, o stream.Options) error {
	return stream.Follow(func() (io.ReadCloser, error) { return FollowLogs(ctxD, id) }, o)
}
