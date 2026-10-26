package proxy

import (
	"context"
	"fmt"
	"time"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/handshake"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

// readyTimeout is the proxy child's deadline to go ready.
const readyTimeout = time.Minute

// ValidateWithChild forks a one-shot proxy child into the pinned netns and
// waits it out — its ready ping proves the egress wall.
func ValidateWithChild(ctx context.Context) error {
	ready, err := handshake.New()
	if err != nil {
		return err
	}
	defer ready.Close()
	cmd, err := NSIsolatedCmd(ctx, "proxy", "child")
	if err != nil {
		return err
	}
	cmd.ExtraFiles = ready.Files()
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("fork proxy child: %w", err)
	}
	ready.Spawned() // the parent never writes the proxy child's end
	if err := ready.AwaitReady(ctx, readyTimeout); err != nil {
		log.WarnErr("kill failed proxy child", cmd.Process.Kill())
		return err
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("proxy child: %w", err)
	}
	return nil
}

// ValidateAndRun proves the egress wall, goes ready, then runs the work — the
// one-shot child's whole life.
func ValidateAndRun(ctx context.Context, work func(context.Context) error) error {
	if err := probeProxy(ctx); err != nil {
		return err
	}
	ready := handshake.ForChild()
	defer ready.Close()
	if err := ready.SendReady(); err != nil {
		return err
	}
	return work(ctx)
}
