package dial

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAwait_Listening(t *testing.T) {
	l := listen(t)
	t.Cleanup(func() { require.NoError(t, l.Close()) })

	require.NoError(t, Await(t.Context(), "tcp", l.Addr().String(), 5*time.Second))
}

func TestAwait_Deadline(t *testing.T) {
	// a port that just closed: every dial refuses, instantly
	l := listen(t)
	addr := l.Addr().String()
	require.NoError(t, l.Close())

	err := Await(t.Context(), "tcp", addr, 10*time.Millisecond)

	require.Error(t, err)
	assert.Contains(t, err.Error(), addr+" never accepted")
}

func TestAwait_Cancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := Await(ctx, "tcp", "127.0.0.1:1", time.Second)

	require.ErrorIs(t, err, context.Canceled)
}

// listen opens a loopback listener — the helper the noctx lint asks for.
func listen(t *testing.T) net.Listener {
	t.Helper()
	l, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	return l
}
