package dial

import (
	"context"
	"net"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAwaitListen_Listening(t *testing.T) {
	if runtime.GOOS != "linux" {
		// listening() is procfs: the connect-free gate has no off-Linux equiv
		t.Skip("skipping off Linux")
	}
	require.NoError(t, AwaitListen(t.Context(), 0, listenPort(t), 5*time.Second))
}

func TestAwaitListen_PidListening(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("skipping off Linux")
	}
	// the pid is this test's own: a real listener under /proc/<pid>/net proves
	// the pid-scoped read without a fork into another netns
	self, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)

	require.NoError(t, AwaitListen(t.Context(), self.Pid, listenPort(t), 5*time.Second))
}

func TestAwaitListen_Deadline(t *testing.T) {
	// a port that just closed: procfs shows no LISTEN on it
	err := AwaitListen(t.Context(), 0, closedPort(t), 10*time.Millisecond)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "never listened")
}

func TestAwaitListen_Cancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := AwaitListen(ctx, 0, 1, time.Second)

	require.ErrorIs(t, err, context.Canceled)
}

// closedPort opens a loopback listener and closes it, answering its port —
// procfs shows no LISTEN on it.
func closedPort(t *testing.T) int {
	t.Helper()
	l, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, portStr, err := net.SplitHostPort(l.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)
	require.NoError(t, l.Close())
	return port
}

// listenPort opens a loopback listener and answers its port — the helper the
// noctx lint asks for.
func listenPort(t *testing.T) int {
	t.Helper()
	l, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, l.Close()) })
	_, portStr, err := net.SplitHostPort(l.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)
	return port
}
