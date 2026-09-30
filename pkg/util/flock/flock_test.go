package flock

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTryEx(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".lock")

	first, err := TryEx(path)
	require.NoError(t, err)

	_, err = TryEx(path)
	require.ErrorIs(t, err, ErrHeld, "a second lock on the same path is held")

	require.NoError(t, first.Release())

	second, err := TryEx(path)
	require.NoError(t, err, "a released lock is reacquirable")
	require.NoError(t, second.Release())
}

func TestTryEx_Errors(t *testing.T) {
	_, err := TryEx(t.TempDir()) // a directory takes no read-write fd
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrHeld, "a real error is not a hold")
}

func TestTryEx_LockFileEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".lock")
	l, err := TryEx(path)
	require.NoError(t, err)
	require.NoError(t, l.Release())

	body, err := os.ReadFile(path) // #nosec G304 -- the test's own lock file
	require.NoError(t, err)
	assert.Empty(t, body, "the lockfile anchors the lock, storing nothing")
}

func TestEx_WaitsForRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".lock")
	first, err := Ex(path)
	require.NoError(t, err)

	type exResult struct {
		l   *Lock
		err error
	}
	acquired := make(chan exResult, 1)
	go func() {
		l, err := Ex(path)
		acquired <- exResult{l, err}
	}()

	select {
	case res := <-acquired:
		require.NoError(t, res.err)
		require.NoError(t, res.l.Release())
		t.Fatal("Ex acquired the lock while it was held")
	case <-time.After(50 * time.Millisecond):
		// still held: Ex waits out the holder
	}

	require.NoError(t, first.Release())
	res := <-acquired
	require.NoError(t, res.err, "Ex acquires once the holder releases")
	require.NoError(t, res.l.Release())
}

func TestEx_LockFileCreated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", ".lock")
	l, err := Ex(path)
	require.NoError(t, err, "the lockfile's parent dir is created")
	require.NoError(t, l.Release())
	assert.FileExists(t, path)
}
