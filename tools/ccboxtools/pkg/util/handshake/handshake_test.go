package handshake

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundTrip(t *testing.T) {
	p, err := New()
	require.NoError(t, err)
	defer p.Close()

	// stand in for the child: Files' end is the child's
	files := p.Files()
	require.Len(t, files, 1)
	_, err = files[0].Write([]byte{'x'}) // SendReady
	require.NoError(t, err)

	require.NoError(t, p.AwaitReady(t.Context(), time.Second))
}

func TestSpawned_EOFsAwaitReady(t *testing.T) {
	p, err := New()
	require.NoError(t, err)
	defer p.Close()

	p.Spawned() // ready's write end closes with this process's copy

	err = p.AwaitReady(t.Context(), time.Second)

	require.ErrorIs(t, err, io.EOF)
	assert.Contains(t, err.Error(), "exited before going ready")
}

func TestAwaitReady_Deadline(t *testing.T) {
	p, err := New()
	require.NoError(t, err)
	defer p.Close()

	err = p.AwaitReady(t.Context(), 10*time.Millisecond)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not ready after")
}

func TestChildRoundTrip(t *testing.T) {
	p, err := New()
	require.NoError(t, err)
	defer p.Close()

	child := &Child{readyW: p.Files()[0]} // the end, as ForChild recovers it

	require.NoError(t, child.SendReady())
	child.Close()
	require.NoError(t, p.AwaitReady(t.Context(), time.Second))
}
