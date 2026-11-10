package supervise

import (
	"context"
	"errors"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_RespawnsDeadChild(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	// each child dies at once, so only Run's SIGCHLD loop can raise the count
	var spawns atomic.Int32
	svc := &Respawnable{
		Name: "test",
		Spawn: func(ctx context.Context) (*exec.Cmd, error) {
			spawns.Add(1)
			return startTrue(ctx)
		},
	}
	require.NoError(t, svc.Start(ctx)) // like the entrypoint's tproxy: the service runs before the supervise

	done := make(chan struct{})
	go func() { Run(ctx, svc); close(done) }()

	assert.Eventually(t, func() bool { return spawns.Load() >= 2 },
		10*time.Second, 10*time.Millisecond, "the dead child was respawned")

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return on the ctx's cancel")
	}
}

func TestRespawnable_Backoff(t *testing.T) {
	r := &Respawnable{Name: "test"}

	assert.Equal(t, respawnBase, r.backoff(), "the first death retries after the base delay")
	assert.Equal(t, 2*respawnBase, r.backoff(), "the next doubles")
	for range 30 {
		r.backoff()
	}
	assert.Equal(t, respawnMax, r.backoff(), "capped")
}

func TestRespawnable_Start_FailedReady(t *testing.T) {
	r := &Respawnable{
		Name:  "test",
		Spawn: startTrue,
		Ready: func(context.Context, *exec.Cmd) error { return errors.New("boom") },
	}

	err := r.Start(t.Context())

	require.Error(t, err)
	assert.Zero(t, r.PID())
}

func TestRespawnable_Reap_Respawns(t *testing.T) {
	r := &Respawnable{Name: "test", Spawn: startTrue}
	require.NoError(t, r.Start(t.Context()))
	pid := r.PID()
	require.NotZero(t, pid)

	r.reap(t.Context(), pid+1) // another child's pid is not ours
	assert.Equal(t, pid, r.PID())

	r.reap(t.Context(), pid)
	assert.Zero(t, r.PID())
	assert.Eventually(t, func() bool {
		return r.PID() != 0 && r.PID() != pid
	}, 5*time.Second, 10*time.Millisecond, "respawned after the backoff")
}

func TestRespawnable_Reap_ResetsBackoff(t *testing.T) {
	r := &Respawnable{Name: "test", Spawn: startTrue}
	require.NoError(t, r.Start(t.Context()))
	r.mu.Lock()
	r.fails = 3                                  // a child that's been flapping
	r.startedAt = time.Now().Add(-respawnResets) // ...but ran durable this time
	r.mu.Unlock()

	r.reap(t.Context(), r.PID())
	r.mu.Lock()
	assert.Equal(t, 1, r.fails, "the durable run reset the flap count; reap's own death counts once")
	r.mu.Unlock()
}

func TestRespawnable_Reap_RetriesFailedRespawns(t *testing.T) {
	// the respawn's first spawn fails: the retry must keep the service down, not the boot
	var fails atomic.Int32
	r := &Respawnable{Name: "test", Spawn: startTrue}
	require.NoError(t, r.Start(t.Context()))
	r.Spawn = func(ctx context.Context) (*exec.Cmd, error) {
		if fails.Add(1) == 1 {
			return nil, errors.New("boom")
		}
		return startTrue(ctx)
	}

	r.reap(t.Context(), r.PID())

	assert.Eventually(t, func() bool { return fails.Load() >= 2 && r.PID() != 0 },
		10*time.Second, 10*time.Millisecond, "the failed respawn was retried until it stuck")
}

func TestRespawnable_Reap_RetriesFailedReady(t *testing.T) {
	// the respawn's first Ready fails: the retry must keep the service down, not the boot
	var fails atomic.Int32
	r := &Respawnable{Name: "test", Spawn: startTrue}
	require.NoError(t, r.Start(t.Context()))
	r.Ready = func(context.Context, *exec.Cmd) error {
		if fails.Add(1) == 1 {
			return errors.New("boom")
		}
		return nil
	}

	r.reap(t.Context(), r.PID())

	assert.Eventually(t, func() bool { return fails.Load() >= 2 && r.PID() != 0 },
		10*time.Second, 10*time.Millisecond, "the failed respawn was retried until it stuck")
}

// startTrue constructs the tests' throwaway child — Start forks it.
func startTrue(_ context.Context) (*exec.Cmd, error) {
	return exec.Command("true"), nil //nolint:gosec,noctx // the tests' throwaway child
}
