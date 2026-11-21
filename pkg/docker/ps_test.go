package docker

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/klean"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/flock"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestHoldersDir(t *testing.T) {
	testutil.Home(t)
	assert.True(t, filepath.IsAbs(HoldersDir("ccbox-proj")), "the holders dir sits under the user dir")
	assert.True(t, strings.HasSuffix(HoldersDir("ccbox-proj"), filepath.Join("devbox-holders", "ccbox-proj")),
		"one holders dir per devbox, keyed by its name")
}

func TestDevboxStatus(t *testing.T) {
	// joinHolder joins the devbox's holder session like a real session would,
	// yielding its leave for the test's cleanup
	joinHolder := func(t *testing.T, holdersDir string) func() error {
		t.Helper()
		noop := func() error { return nil }
		leave, err := flock.MultiFlock{Dir: holdersDir}.Join(noop, noop, noop)
		require.NoError(t, err)
		return leave
	}
	summary := func(state string) container.Summary {
		return container.Summary{Names: []string{"/ccbox-proj"}, State: state}
	}

	t.Run("StoppedHoldsNoSession", func(t *testing.T) {
		testutil.Home(t)
		status, err := devboxStatus(summary("exited"))
		require.NoError(t, err)
		assert.Equal(t, DevboxStatus{Name: "ccbox-proj", State: "exited"}, status)
	})

	t.Run("RunningWithoutSessions", func(t *testing.T) {
		testutil.Home(t)
		status, err := devboxStatus(summary("running"))
		require.NoError(t, err)
		assert.Equal(t, DevboxStatus{Name: "ccbox-proj", State: "running"}, status)
	})

	t.Run("ForegroundCountsBeyondItsExecs", func(t *testing.T) {
		testutil.Home(t)
		o := RunOptions{Name: "ccbox-proj", HoldersDir: HoldersDir("ccbox-proj")}
		release, err := takeForegroundMark(o)
		require.NoError(t, err)
		defer func() { require.NoError(t, release()) }()

		leaves := make([]func() error, 0, 3) // the foreground run + its two execs
		for range 3 {
			leaves = append(leaves, joinHolder(t, o.HoldersDir))
		}
		t.Cleanup(func() {
			for _, leave := range leaves {
				require.NoError(t, klean.SwallowErr(leave, flock.ErrNotLast)())
			}
		})

		status, err := devboxStatus(summary("running"))
		require.NoError(t, err)
		assert.Equal(t, DevboxStatus{Name: "ccbox-proj", State: "running", Execs: 2, Foreground: true}, status)
	})

	t.Run("HolderWithoutMarkCountsAsOneExec", func(t *testing.T) {
		testutil.Home(t)
		o := RunOptions{Name: "ccbox-proj", HoldersDir: HoldersDir("ccbox-proj")}
		leave := joinHolder(t, o.HoldersDir)
		t.Cleanup(func() { require.NoError(t, klean.SwallowErr(leave, flock.ErrNotLast)()) })

		status, err := devboxStatus(summary("running"))
		require.NoError(t, err)
		assert.Equal(t, DevboxStatus{Name: "ccbox-proj", State: "running", Execs: 1}, status,
			"the run joins before it takes its mark: the boot window counts it as an exec, never below zero")
	})
}

func TestIsForegrounded(t *testing.T) {
	markPath := filepath.Join(t.TempDir(), "foreground")

	foreground, err := isForegrounded(markPath)
	require.NoError(t, err)
	assert.False(t, foreground, "a free mark foregrounds no one")

	mark, err := flock.TryEx(markPath)
	require.NoError(t, err)
	defer func() { require.NoError(t, mark.Release()) }()

	foreground, err = isForegrounded(markPath)
	require.NoError(t, err)
	assert.True(t, foreground)
}
