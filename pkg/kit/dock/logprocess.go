package dock

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types/container"

	"github.com/s12chung/ccbox/pkg/util/stream"
	"github.com/s12chung/ccbox/pkg/util/term"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/errs"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

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
