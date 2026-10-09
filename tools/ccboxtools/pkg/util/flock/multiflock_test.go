package flock

import (
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMultiFlock(t *testing.T) MultiFlock {
	t.Helper()
	return MultiFlock{Dir: t.TempDir()}
}

// existingEntry creates an existing holder's entry: a file flock-held by this process,
// as a concurrent holder holds
func existingEntry(t *testing.T, m MultiFlock) {
	t.Helper()
	f, err := os.CreateTemp(m.Dir, entryPrefix)
	require.NoError(t, err)
	l, err := TryEx(f.Name())
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, l.Release()) })
}

// deadEntry creates a dead holder's entry: a file no one flocks, as a crashed holder
// leaves behind
func deadEntry(t *testing.T, m MultiFlock) string {
	t.Helper()
	f, err := os.CreateTemp(m.Dir, entryPrefix)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

func entryNames(t *testing.T, m MultiFlock) []string {
	t.Helper()
	entries, err := os.ReadDir(m.Dir)
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), entryPrefix) {
			names = append(names, e.Name())
		}
	}
	return names
}

func TestMultiFlock_Join_Solo(t *testing.T) {
	m := newMultiFlock(t)

	fnRan, validateRan := false, false
	leave, err := m.Join(
		func() error { validateRan = true; return nil },
		func() error { fnRan = true; return nil },
		nil,
	)
	require.NoError(t, err)
	assert.True(t, fnRan, "the holder is the resource's first: it creates")
	assert.False(t, validateRan)
	require.Len(t, entryNames(t, m), 1, "the holder's own entry is registered")

	require.NoError(t, leave())
	assert.Empty(t, entryNames(t, m), "the entry is dropped on leave")
}

func TestMultiFlock_Join_Existing(t *testing.T) {
	m := newMultiFlock(t)
	existingEntry(t, m)

	fnRan, validateRan := false, false
	leave, err := m.Join(
		func() error { validateRan = true; return nil },
		func() error { fnRan = true; return nil },
		nil,
	)
	require.NoError(t, err)
	assert.True(t, validateRan, "the held entry counts as an existing holder: verify, don't create")
	assert.False(t, fnRan)

	require.ErrorIs(t, leave(), ErrNotLast) // the existing holder keeps the calling holder from being last
	assert.Len(t, entryNames(t, m), 1, "the existing entry is kept")
}

func TestMultiFlock_Join_CallbackUnderLock(t *testing.T) {
	check := func(m MultiFlock) bool {
		held := false
		callback := func() error {
			_, lockErr := TryEx(m.lockPath())
			held = errors.Is(lockErr, ErrHeld)
			return nil
		}
		_, err := m.Join(callback, callback, nil)
		require.NoError(t, err)
		return held
	}

	assert.True(t, check(newMultiFlock(t)), "fn runs under the dir lock")

	m := newMultiFlock(t)
	existingEntry(t, m)
	assert.True(t, check(m), "validateExistingFn runs under the dir lock")
}

func TestMultiFlock_Join_SweepsDeadEntries(t *testing.T) {
	m := newMultiFlock(t)
	dead := deadEntry(t, m)

	fnRan := false
	leave, err := m.Join(func() error { return nil }, func() error { fnRan = true; return nil }, nil)
	require.NoError(t, err)
	assert.True(t, fnRan, "the dead entry is not an existing holder: the holder creates")
	assert.NoFileExists(t, dead, "the dead entry is swept")
	require.Len(t, entryNames(t, m), 1, "only the holder's own entry remains")

	require.NoError(t, leave())
}

func TestMultiFlock_Leave_SweepsDeadEntries(t *testing.T) {
	m := newMultiFlock(t)

	leave, err := m.Join(func() error { return nil }, func() error { return nil }, nil)
	require.NoError(t, err)
	deadEntry(t, m)

	require.NoError(t, leave())
	assert.Empty(t, entryNames(t, m))
}

func TestMultiFlock_Leave_Teardown_LastOnly(t *testing.T) {
	m := newMultiFlock(t)

	teardowns := 0
	first, err := m.Join(func() error { return nil }, func() error { return nil }, func() error { teardowns++; return nil })
	require.NoError(t, err)
	second, err := m.Join(func() error { return nil }, func() error { return nil }, func() error { teardowns++; return nil })
	require.NoError(t, err)

	require.ErrorIs(t, first(), ErrNotLast)
	assert.Zero(t, teardowns, "an existing holder remains: its leave skips teardown")
	require.NoError(t, second())
	assert.Equal(t, 1, teardowns, "the last one out tears the resource down")
}

func TestMultiFlock_Leave_Teardown_UnderLock(t *testing.T) {
	m := newMultiFlock(t)

	held := false
	leave, err := m.Join(func() error { return nil }, func() error { return nil }, func() error {
		_, lockErr := TryEx(m.lockPath())
		held = errors.Is(lockErr, ErrHeld)
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, leave())
	assert.True(t, held, "teardown runs under the dir lock: a blocked join sees no half state")
}

func TestMultiFlock_Leave_Teardown_ErrorStillDrops(t *testing.T) {
	m := newMultiFlock(t)
	wantErr := errors.New("boom")

	leave, err := m.Join(func() error { return nil }, func() error { return nil }, func() error { return wantErr })
	require.NoError(t, err)

	require.ErrorIs(t, leave(), wantErr)
	assert.Empty(t, entryNames(t, m), "a failed teardown must not hold the count hostage")

	fnRan := false
	_, err = m.Join(func() error { return nil }, func() error { fnRan = true; return nil }, nil)
	require.NoError(t, err)
	assert.True(t, fnRan, "the dropped entry lets the next holder create anew")
}

func TestMultiFlock_Join_CallbackError(t *testing.T) {
	wantErr := errors.New("boom")

	t.Run("Fn", func(t *testing.T) {
		m := newMultiFlock(t)
		_, err := m.Join(func() error { return nil }, func() error { return wantErr }, nil)
		require.ErrorIs(t, err, wantErr)
		assert.Empty(t, entryNames(t, m), "a failed create registers no entry")
	})

	t.Run("ValidateExistingFn", func(t *testing.T) {
		m := newMultiFlock(t)
		existingEntry(t, m)
		_, err := m.Join(func() error { return wantErr }, func() error { return nil }, nil)
		require.ErrorIs(t, err, wantErr)
		assert.Len(t, entryNames(t, m), 1, "only the existing entry remains")
	})
}

func TestMultiFlock_Join_Concurrent(t *testing.T) {
	m := newMultiFlock(t)
	const holders = 8

	var fns, validates atomic.Int32
	var mu sync.Mutex
	leaves := make([]func() error, 0, holders)
	var joins sync.WaitGroup
	for range holders {
		joins.Go(func() {
			leave, err := m.Join(
				func() error { validates.Add(1); return nil },
				func() error { fns.Add(1); return nil },
				nil,
			)
			if err != nil {
				t.Errorf("join: %v", err)
				return
			}
			mu.Lock()
			leaves = append(leaves, leave)
			mu.Unlock()
		})
	}
	joins.Wait()
	assert.Equal(t, int32(1), fns.Load(), "exactly one holder created the resource")
	assert.Equal(t, int32(holders-1), validates.Load(), "every other holder joined the existing ones")

	// every holder leaves: all but the last out find an existing holder among the rest
	var notLasts atomic.Int32
	var leavesDone sync.WaitGroup
	for _, leave := range leaves {
		leavesDone.Go(func() {
			err := leave()
			switch {
			case errors.Is(err, ErrNotLast):
				notLasts.Add(1)
			case err != nil:
				t.Errorf("leave: %v", err)
			}
		})
	}
	leavesDone.Wait()
	assert.Equal(t, int32(holders-1), notLasts.Load(), "every holder but the last out leaves an existing holder behind")
	assert.Empty(t, entryNames(t, m), "every entry is dropped")
}

func TestMultiFlock_Clean_Solo(t *testing.T) {
	m := newMultiFlock(t)

	release, existing, err := m.Clean()
	require.NoError(t, err)
	assert.Equal(t, 0, existing)
	assert.Empty(t, entryNames(t, m))
	release()
}

func TestMultiFlock_Clean_Existing(t *testing.T) {
	m := newMultiFlock(t)
	existingEntry(t, m)

	release, existing, err := m.Clean()
	require.NoError(t, err)
	assert.Equal(t, 1, existing, "the held entry counts as an existing holder")
	assert.Len(t, entryNames(t, m), 1, "the existing entry is kept")
	release()
}

func TestMultiFlock_Clean_SweepsDeadEntries(t *testing.T) {
	m := newMultiFlock(t)
	dead := deadEntry(t, m)

	release, existing, err := m.Clean()
	require.NoError(t, err)
	assert.Equal(t, 0, existing, "the dead entry is not an existing holder")
	assert.NoFileExists(t, dead, "the dead entry is swept")
	release()
}

func TestMultiFlock_Clean_HoldsDirLock(t *testing.T) {
	m := newMultiFlock(t)

	release, _, err := m.Clean()
	require.NoError(t, err)
	_, lockErr := TryEx(m.lockPath())
	require.ErrorIs(t, lockErr, ErrHeld, "the dir lock is held until release")
	release()
	_, lockErr = TryEx(m.lockPath())
	require.NoError(t, lockErr, "release lets the dir lock go")
}
