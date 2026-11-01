package dial

import (
	"context"
	"net"
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
	require.NoError(t, AwaitListen(t.Context(), listenPort(t), 5*time.Second))
}

func TestAwaitListen_Deadline(t *testing.T) {
	// a port that just closed: procfs shows no LISTEN on it
	l, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, portStr, err := net.SplitHostPort(l.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)
	require.NoError(t, l.Close())

	err = AwaitListen(t.Context(), port, 10*time.Millisecond)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "never listened")
}

func TestAwaitListen_Cancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := AwaitListen(ctx, 1, time.Second)

	require.ErrorIs(t, err, context.Canceled)
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
