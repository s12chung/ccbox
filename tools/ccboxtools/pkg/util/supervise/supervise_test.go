package supervise

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRespawnable_Backoff(t *testing.T) {
	r := &Respawnable{Name: "test"}

	assert.Equal(t, respawnBase, r.backoff(), "the first death retries after the base delay")
	assert.Equal(t, 2*respawnBase, r.backoff(), "the next doubles")
	for range 30 {
		r.backoff()
	}
	assert.Equal(t, respawnMax, r.backoff(), "capped")
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

// startTrue spawns the reaper test's throwaway child.
func startTrue(_ context.Context) (*exec.Cmd, error) {
	cmd := exec.Command("true") //nolint:gosec,noctx // the reaper test's throwaway child
	return cmd, cmd.Start()
}
