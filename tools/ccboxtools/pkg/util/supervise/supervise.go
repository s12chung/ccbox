// Package supervise watches child processes: when one dies, it's brought back
// after a backoff. Run returns on the ctx's cancel.
package supervise

import (
	"context"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/proc"
)

// The respawn backoff: double per death, capped, reset by a durable run.
const (
	respawnBase   = time.Second
	respawnMax    = 30 * time.Second
	respawnResets = time.Minute // alive this long → the next death restarts the backoff
)

// Run supervises svcs until the ctx's cancel. SIGCHLD wakes it as children
// die.
func Run(ctx context.Context, svcs ...*Respawnable) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGCHLD)
	defer signal.Stop(sig)
	reapAll(ctx, svcs) // a child may have died before the watch began
	for {
		select {
		case <-ctx.Done():
			return
		case <-sig:
			reapAll(ctx, svcs)
		}
	}
}

// RunF brings f's services up and supervises them — f's error fails the caller.
func RunF(ctx context.Context, f func(ctx context.Context) ([]*Respawnable, error)) error {
	svcs, err := f(ctx)
	if err != nil {
		return err
	}
	Run(ctx, svcs...)
	return nil
}

// reapAll drains every exited child without blocking, adopting each into its
// service — docker exec's re-parented zombies included.
func reapAll(ctx context.Context, svcs []*Respawnable) {
	for {
		_, pid := proc.ReapOne()
		if pid <= 0 {
			return
		}
		for _, svc := range svcs {
			svc.reap(ctx, pid)
		}
	}
}

// Respawnable is a supervised child brought up and kept up: spawned at boot,
// respawned with a backoff on death.
type Respawnable struct {
	Name  string
	Spawn func(ctx context.Context) (*exec.Cmd, error)

	mu        sync.Mutex
	cmd       *exec.Cmd
	startedAt time.Time
	fails     int
}

// Start spawns the service's child.
func (r *Respawnable) Start(ctx context.Context) error {
	cmd, err := r.Spawn(ctx)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.cmd, r.startedAt = cmd, time.Now()
	r.mu.Unlock()
	return nil
}

// PID is the child's pid, 0 when it isn't running.
func (r *Respawnable) PID() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cmd == nil {
		return 0
	}
	return r.cmd.Process.Pid
}

// reap adopts a reaped child pid when it is this service's, respawning after
// a backoff.
func (r *Respawnable) reap(ctx context.Context, pid int) {
	r.mu.Lock()
	if r.cmd == nil || r.cmd.Process.Pid != pid {
		r.mu.Unlock()
		return
	}
	r.cmd = nil
	if time.Since(r.startedAt) >= respawnResets {
		r.fails = 0
	}
	delay := r.backoffLocked()
	r.mu.Unlock()

	go r.respawn(ctx, delay)
}

// respawn keeps the service up: wait out each backoff, spawn, retry until it
// sticks — a failing respawn degrades the caller without killing it.
func (r *Respawnable) respawn(ctx context.Context, delay time.Duration) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		err := r.Start(ctx)
		if err == nil {
			return
		}
		log.Warnf("%s respawn failed: %v", r.Name, err)
		delay = r.backoff()
	}
}

func (r *Respawnable) backoff() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.backoffLocked()
}

func (r *Respawnable) backoffLocked() time.Duration {
	r.fails++
	return min(respawnMax, respawnBase<<min(r.fails-1, 16))
}
