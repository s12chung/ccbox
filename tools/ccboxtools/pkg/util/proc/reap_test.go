package proc

import (
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReapOne(t *testing.T) {
	cmd := exec.Command("true") //nolint:gosec,noctx // the reaper's throwaway child
	require.NoError(t, cmd.Start())
	pid := cmd.Process.Pid

	var reaped int
	require.Eventually(t, func() bool {
		_, reaped = ReapOne()
		return reaped == pid
	}, 5*time.Second, 10*time.Millisecond, "reaps the exited child")
}
