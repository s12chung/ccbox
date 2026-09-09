package lock

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hold takes an uncontended lock the way Do does, for the test to contend against.
func hold(t *testing.T, lockPath string) func() {
	t.Helper()
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, fileMode) // #nosec G304 -- the test's own lock file
	require.NoError(t, err)
	fd := int(f.Fd()) // resolved eagerly: release may run against the cleanup's Close
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Errorf("close lock file: %v", err)
		}
	})
	// take the lock for the test to hold; uncontended, so no waiting
	require.NoError(t, syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB))
	return func() {
		// release the lock
		if err := syscall.Flock(fd, syscall.LOCK_UN); err != nil {
			t.Errorf("unlock: %v", err)
		}
	}
}

func TestDo(t *testing.T) {
	cases := []struct {
		name string
		held bool
	}{
		{"Uncontended", false},
		{"Held_Skip", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ran, skipCalled bool
			lockPath := filepath.Join(t.TempDir(), "claude.lock")
			if tc.held {
				release := hold(t, lockPath)
				defer release()
			}

			err := Do(lockPath, func() error { ran = true; return nil }, func() bool {
				skipCalled = true
				return true
			})

			require.NoError(t, err)
			assert.Equal(t, !tc.held, ran)       // fn runs only uncontended
			assert.Equal(t, tc.held, skipCalled) // skip runs only when held
		})
	}
}

func TestDo_Held_NoSkip_WaitsThenRunsFn(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "claude.lock")
	release := hold(t, lockPath)

	var mu sync.Mutex
	var order []string
	step := func(s string) { mu.Lock(); defer mu.Unlock(); order = append(order, s) }

	skipped := make(chan struct{})
	go func() {
		<-skipped // Do's non-blocking try hit the held lock
		release()
	}()

	err := Do(lockPath, func() error { step("fn"); return nil }, func() bool {
		step("skip")
		close(skipped)
		return false
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"skip", "fn"}, order)
}

func TestDo_FnError_PropagatesAndReleases(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "claude.lock")
	wantErr := errors.New("boom")

	err := Do(lockPath, func() error { return wantErr }, func() bool {
		t.Fatal("skip consulted on an uncontended lock")
		return true
	})

	require.ErrorIs(t, err, wantErr)
	release := hold(t, lockPath) // fails the require, never hangs, on a leaked lock
	release()
}

func TestDo_ReleasesAfterRun(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "claude.lock")
	fn := func() error { return nil }

	require.NoError(t, Do(lockPath, fn, func() bool { return true }))

	var ran bool // a second Do takes the lock rather than skipping to it
	require.NoError(t, Do(lockPath, func() error { ran = true; return nil }, func() bool { return true }))
	assert.True(t, ran)
}
