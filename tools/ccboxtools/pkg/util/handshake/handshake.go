// Package handshake wires a fork's one-byte ready signal: the child pings once
// its side is proven, and the parent's await doubles as a liveness check — a
// dead peer's end reads EOF.
package handshake

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// readyFd is the signal's slot in the child's fd table — the first after
// stdin, stdout, and stderr: ExtraFiles assigns to fds sequentially from there.
const readyFd = 3

// Parent is the fork's parent half: the child's end until it runs, plus its
// own reading end.
type Parent struct {
	readyR *os.File // this process's end
	readyW *os.File // the child's end, until the spawn
}

// New creates the ready signal.
func New() (*Parent, error) {
	readyR, readyW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	return &Parent{readyR: readyR, readyW: readyW}, nil
}

// Files are the child's ends: append them to exec.Cmd's ExtraFiles — the first
// lands at readyFd, where ForChild recovers it.
func (p *Parent) Files() []*os.File {
	return []*os.File{p.readyW}
}

// Spawned closes this process's copy of the child's end — the child got its
// own at the fork. Call it once the child is running.
func (p *Parent) Spawned() {
	log.WarnErr("close ready pipe", p.readyW.Close())
	p.readyW = nil
}

// Close closes every end, the child's copy included — safe however far the
// handshake got.
func (p *Parent) Close() {
	if p.readyW != nil {
		p.Spawned()
	}
	if p.readyR != nil {
		log.WarnErr("close ready pipe", p.readyR.Close())
		p.readyR = nil
	}
}

// AwaitReady blocks for the child's ping, its death (EOF), the ctx's cancel,
// or the deadline — only the ping is nil.
func (p *Parent) AwaitReady(ctx context.Context, timeout time.Duration) error {
	pong := make(chan error, 1)
	go func() {
		_, err := p.readyR.Read(make([]byte, 1)) // the ping, or EOF on the child's death
		if err != nil {
			err = fmt.Errorf("wait ready: %w", err)
		}
		pong <- err
	}()
	select {
	case err := <-pong:
		if err != nil {
			return fmt.Errorf("child exited before going ready: %w", err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("child not ready: %w", ctx.Err())
	case <-time.After(timeout):
		return fmt.Errorf("child not ready after %s", timeout)
	}
}

// Child is the fork's child half, recovered from its inherited fd.
type Child struct {
	readyW *os.File
}

// ForChild recovers the signal from this process's fd table.
func ForChild() *Child {
	return &Child{readyW: os.NewFile(readyFd, "ready")} //nolint:gosec // the parent's pinned pipe fd
}

// SendReady pings the parent: this side is proven.
func (c *Child) SendReady() error {
	if _, err := c.readyW.Write([]byte{'x'}); err != nil {
		return fmt.Errorf("send ready: %w", err)
	}
	return nil
}

// Close closes the end.
func (c *Child) Close() {
	log.WarnErr("close ready pipe", c.readyW.Close())
}
