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

// forkProxyChild forks a one-shot proxy child into the pinned netns and
// waits out `ccbox proxy child` — its Validate run proves the egress wall.
func forkProxyChild(ctx context.Context) error {
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
		log.WarnErr("reap failed proxy child", cmd.Wait())
		return err
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("proxy child: %w", err)
	}
	return nil
}

// Validate proves the egress wall and goes ready — the one-shot child's arrival proof.
func Validate(ctx context.Context) error {
	if err := probeProxy(ctx); err != nil {
		return err
	}
	ready := handshake.ForChild()
	defer ready.Close()
	return ready.SendReady()
}
