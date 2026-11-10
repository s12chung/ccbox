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
		CmdToStart: func(ctx context.Context) (*exec.Cmd, error) {
			spawns.Add(1)
			return startTrue(ctx)
		},
	}
	require.NoError(t, svc.Start(ctx)) // like the entrypoint's tproxy: the service runs before the supervise

	runErr := make(chan error, 1)
	go func() { runErr <- Run(ctx, nil, svc) }()

	assert.Eventually(t, func() bool { return spawns.Load() >= 2 },
		10*time.Second, 10*time.Millisecond, "the dead child was respawned")

	cancel()
	select {
	case err := <-runErr:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return on the ctx's cancel")
	}
}

func TestRun_AfterStart_AfterSvcsStart(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	gate := make(chan struct{})
	svc := &Respawnable{
		Name:       "test",
		CmdToStart: startTrue,
		Ready: func(context.Context, *exec.Cmd) error {
			<-gate
			return nil
		},
	}
	var fired atomic.Bool
	runErr := make(chan error, 1)
	go func() { runErr <- Run(ctx, func() { fired.Store(true) }, svc) }()

	require.Never(t, fired.Load,
		100*time.Millisecond, 10*time.Millisecond, "afterStart fired while the service's gate was pending")

	close(gate)
	assert.Eventually(t, fired.Load, 5*time.Second, 10*time.Millisecond,
		"afterStart fires once every service has started")

	cancel()
	require.NoError(t, <-runErr)
}

func TestRun_AfterStart_Nil(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	svc := &Respawnable{Name: "test", CmdToStart: startSleep}
	runErr := make(chan error, 1)
	go func() { runErr <- Run(ctx, nil, svc) }()

	assert.Eventually(t, func() bool { return pidOf(svc) != 0 },
		5*time.Second, 10*time.Millisecond, "Run started the service with no afterStart")

	cancel()
	require.NoError(t, <-runErr)
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
		Name:       "test",
		CmdToStart: startTrue,
		Ready:      func(context.Context, *exec.Cmd) error { return errors.New("boom") },
	}

	err := r.Start(t.Context())

	require.Error(t, err)
	assert.Zero(t, pidOf(r))
}

func TestRespawnable_Reap_Respawns(t *testing.T) {
	r := &Respawnable{Name: "test", CmdToStart: startTrue}
	require.NoError(t, r.Start(t.Context()))
	pid := pidOf(r)
	require.NotZero(t, pid)

	r.reap(t.Context(), pid+1) // another child's pid is not ours
	assert.Equal(t, pid, pidOf(r))

	r.reap(t.Context(), pid)
	assert.Zero(t, pidOf(r))
	assert.Eventually(t, func() bool {
		return pidOf(r) != 0 && pidOf(r) != pid
	}, 5*time.Second, 10*time.Millisecond, "respawned after the backoff")
}

func TestRespawnable_Reap_ResetsBackoff(t *testing.T) {
	r := &Respawnable{Name: "test", CmdToStart: startTrue}
	require.NoError(t, r.Start(t.Context()))
	r.mu.Lock()
	r.fails = 3                                  // a child that's been flapping
	r.startedAt = time.Now().Add(-respawnResets) // ...but ran durable this time
	r.mu.Unlock()

	r.reap(t.Context(), pidOf(r))
	r.mu.Lock()
	assert.Equal(t, 1, r.fails, "the durable run reset the flap count; reap's own death counts once")
	r.mu.Unlock()
}

func TestRespawnable_Reap_RetriesFailedRespawns(t *testing.T) {
	// the respawn's first spawn fails: the retry must keep the service down, not the boot
	var fails atomic.Int32
	r := &Respawnable{Name: "test", CmdToStart: startTrue}
	require.NoError(t, r.Start(t.Context()))
	r.CmdToStart = func(ctx context.Context) (*exec.Cmd, error) {
		if fails.Add(1) == 1 {
			return nil, errors.New("boom")
		}
		return startTrue(ctx)
	}

	r.reap(t.Context(), pidOf(r))

	assert.Eventually(t, func() bool { return fails.Load() >= 2 && pidOf(r) != 0 },
		10*time.Second, 10*time.Millisecond, "the failed respawn was retried until it stuck")
}

func TestRespawnable_Reap_RetriesFailedReady(t *testing.T) {
	// the respawn's first Ready fails: the retry must keep the service down, not the boot
	var fails atomic.Int32
	r := &Respawnable{Name: "test", CmdToStart: startTrue}
	require.NoError(t, r.Start(t.Context()))
	r.Ready = func(context.Context, *exec.Cmd) error {
		if fails.Add(1) == 1 {
			return errors.New("boom")
		}
		return nil
	}

	r.reap(t.Context(), pidOf(r))

	assert.Eventually(t, func() bool { return fails.Load() >= 2 && pidOf(r) != 0 },
		10*time.Second, 10*time.Millisecond, "the failed respawn was retried until it stuck")
}

// startTrue constructs the tests' throwaway child — Start forks it.
func startTrue(_ context.Context) (*exec.Cmd, error) {
	return exec.Command("true"), nil //nolint:gosec,noctx // the tests' throwaway child
}

// startSleep constructs the tests' throwaway child that lives — Start forks it.
func startSleep(_ context.Context) (*exec.Cmd, error) {
	return exec.Command("sleep", "30"), nil //nolint:gosec,noctx // the tests' throwaway child
}

// pidOf is the service's child's pid, 0 when it isn't running — the tests'
// window onto the unexported cmd.
func pidOf(r *Respawnable) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cmd == nil {
		return 0
	}
	return r.cmd.Process.Pid
}
