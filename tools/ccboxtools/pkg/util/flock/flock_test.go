package flock

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
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

func TestHold(t *testing.T) {
	dir := t.TempDir() // a held version's own dir: no lockfile beside it

	first, err := Hold(dir)
	require.NoError(t, err)
	second, err := Hold(dir)
	require.NoError(t, err, "holds on the same path coexist")
	assert.True(t, Held(dir))

	require.NoError(t, first())
	require.NoError(t, second())
	assert.False(t, Held(dir), "released with its holders")
}

func TestHoldExec(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, HoldExec(dir))
	assert.True(t, Held(dir), "held across the exec's process")

	hold, err := Hold(dir)
	require.NoError(t, err, "coexists with the other holders")
	require.NoError(t, hold())

	err = HoldExec(filepath.Join(t.TempDir(), "absent"))
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist, "a missing path is never created")
}

const (
	holdExecChildEnv     = "FLOCK_TEST_HOLD_EXEC_PATH"
	holdExecConflictExit = 42 // flock's -E: apart from the probe's own failures, which exit 1
)

func TestHoldExec_SurvivesExec(t *testing.T) {
	if heldPath := os.Getenv(holdExecChildEnv); heldPath != "" {
		holdExecExecProbe(t, heldPath) // execs the probe; never returns
		return
	}
	if _, err := exec.LookPath("flock"); err != nil {
		t.Skipf("flock probe unavailable: %v", err)
	}
	// only the child holds — a parent's hold would leak into the probe — and the
	// probe's conflict exit proves the hold survived the exec
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+t.Name()+"$") // #nosec G204,G702 -- re-execs the test binary itself
	cmd.Env = append(os.Environ(), holdExecChildEnv+"="+t.TempDir())
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr, "the exec'd probe found the path unheld: %s", out)
	assert.Equal(t, holdExecConflictExit, exitErr.ExitCode(), "flock's conflict exit, not the probe's own failure: %s", out)
}

// holdExecExecProbe takes the hold then execs flock's non-blocking probe — the
// probe's conflict exit proves the hold survived the exec. Never returns.
func holdExecExecProbe(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, HoldExec(path))
	flockBin, err := exec.LookPath("flock")
	require.NoError(t, err)
	argv := []string{"flock", "-n", "-x", "-E", strconv.Itoa(holdExecConflictExit), path, "-c", "true"}
	require.NoError(t, syscall.Exec(flockBin, argv, os.Environ())) // #nosec G204,G702 -- the test's own probe; success replaces the process
}

func TestEx_LockFileCreated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", ".lock")
	l, err := Ex(path)
	require.NoError(t, err, "the lockfile's parent dir is created")
	require.NoError(t, l.Release())
	assert.FileExists(t, path)
}

func TestHeld(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".lock")

	assert.False(t, Held(path), "a missing path is never held")
	assert.NoFileExists(t, path, "the probe creates no lockfile")
	require.NoError(t, os.WriteFile(path, nil, fileMode))

	ex, err := Ex(path)
	require.NoError(t, err)
	assert.True(t, Held(path), "held under an exclusive holder")
	require.NoError(t, ex.Release())
	assert.False(t, Held(path))
}

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

func TestDo_RunsFnAndReleases(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "claude.lock")
	var ran bool
	require.NoError(t, Do(t.Context(), lockPath, func() error { ran = true; return nil }, nil))

	assert.True(t, ran)
	release := hold(t, lockPath) // fails the require, never hangs, on a leaked lock
	release()
}

func TestDo_Held_Skips(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "claude.lock")
	release := hold(t, lockPath)
	defer release()

	var ran bool
	err := Do(t.Context(), lockPath, func() error { ran = true; return nil }, func() bool { return true })

	require.NoError(t, err)
	assert.False(t, ran, "a skip's true leaves fn unrun, the lock unwaited")
}

func TestDo_Held_WaitsThenRunsFn(t *testing.T) {
	cases := []struct {
		name string
		skip func() bool
	}{
		{"NilSkip", nil},
		{"SkipFalse", func() bool { return false }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lockPath := filepath.Join(t.TempDir(), "claude.lock")
			release := hold(t, lockPath)

			done := make(chan error, 1)
			go func() { done <- Do(t.Context(), lockPath, func() error { return nil }, tc.skip) }()

			assert.Never(t, func() bool { return len(done) > 0 },
				100*time.Millisecond, 10*time.Millisecond, "Do returned while the lock was held")

			release()
			select {
			case err := <-done:
				require.NoError(t, err, "Do runs fn once the holder releases")
			case <-time.After(5 * time.Second):
				t.Fatal("Do did not return after the release")
			}
		})
	}
}

func TestDo_FnError_PropagatesAndReleases(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "claude.lock")
	wantErr := errors.New("boom")

	err := Do(t.Context(), lockPath, func() error { return wantErr }, nil)

	require.ErrorIs(t, err, wantErr)
	release := hold(t, lockPath) // fails the require, never hangs, on a leaked lock
	release()
}
