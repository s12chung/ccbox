package install

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkger"
)

// fakePkger serves a canned latest version; Install simulates npm's bin layout.
type fakePkger struct {
	name      string
	latest    string
	latestErr error

	installed []string
}

func (f *fakePkger) Name() string            { return f.name }
func (f *fakePkger) Latest() (string, error) { return f.latest, f.latestErr }
func (f *fakePkger) RelBin() string          { return "bin/" + f.name }
func (f *fakePkger) Install(dir, version string) error {
	f.installed = append(f.installed, version)
	if err := os.MkdirAll(filepath.Join(dir, "bin"), dirMode); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "bin", f.name), nil, dirMode)
}

func readLink(t *testing.T, path string) string {
	t.Helper()
	target, err := os.Readlink(path)
	require.NoError(t, err)
	return target
}

func entryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// holdLock takes an uncontended install lock the way lock.Do does, for the test
// to contend against.
func holdLock(t *testing.T, lockPath string) func() {
	t.Helper()
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600) // #nosec G304 -- the test's own lock file
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

func TestRun_Installs(t *testing.T) {
	p := &fakePkger{name: "claude", latest: "1.2.3"}
	root := t.TempDir()

	require.NoError(t, Run(pkger.PkgDir{Pkger: p, Root: root}))

	// version dir in place, current flipped, bin exposed
	require.DirExists(t, filepath.Join(root, "claude", "1.2.3"))
	assert.Equal(t, "1.2.3", readLink(t, filepath.Join(root, "claude", "current")))
	assert.Equal(t,
		filepath.Join("..", "claude", "current", "bin", "claude"),
		readLink(t, filepath.Join(root, "bin", "claude")))
	assert.Equal(t, []string{"1.2.3", "current"}, entryNames(t, filepath.Join(root, "claude")))
	// the lock file is root-level, out of prune's reach
	assert.FileExists(t, filepath.Join(root, "claude.lock"))
}

func TestRun_UpgradesAndPrunes(t *testing.T) {
	p := &fakePkger{name: "claude", latest: "1.2.3"}
	root := t.TempDir()
	require.NoError(t, Run(pkger.PkgDir{Pkger: p, Root: root}))

	p.latest = "2.0.0"
	require.NoError(t, Run(pkger.PkgDir{Pkger: p, Root: root}))

	assert.Equal(t, []string{"1.2.3", "2.0.0"}, p.installed)
	assert.Equal(t, "2.0.0", readLink(t, filepath.Join(root, "claude", "current")))
	assert.Equal(t, []string{"2.0.0", "current"}, entryNames(t, filepath.Join(root, "claude")))
}

func TestRun_UpToDate(t *testing.T) {
	p := &fakePkger{name: "claude", latest: "1.2.3"}
	root := t.TempDir()
	require.NoError(t, Run(pkger.PkgDir{Pkger: p, Root: root}))
	require.NoError(t, Run(pkger.PkgDir{Pkger: p, Root: root}))

	assert.Equal(t, []string{"1.2.3"}, p.installed) // no re-install on the second run
}

func TestRun_ChannelFails_FallsBackToStale(t *testing.T) {
	p := &fakePkger{name: "claude", latest: "1.2.3"}
	root := t.TempDir()
	require.NoError(t, Run(pkger.PkgDir{Pkger: p, Root: root}))

	p.latest, p.latestErr = "", errors.New("channel down")
	require.NoError(t, Run(pkger.PkgDir{Pkger: p, Root: root})) // the installed 1.2.3 stands in

	assert.Equal(t, []string{"1.2.3"}, p.installed)
	assert.Equal(t, "1.2.3", readLink(t, filepath.Join(root, "claude", "current")))
}

func TestRun_ChannelFails_NothingInstalled(t *testing.T) {
	p := &fakePkger{name: "claude", latest: "", latestErr: errors.New("channel down")}

	err := Run(pkger.PkgDir{Pkger: p, Root: t.TempDir()})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "channel down")
}

func TestRun_LockHeld(t *testing.T) {
	cases := []struct {
		name          string
		waitEnv       string
		preInstalled  string
		runHeld       bool
		latest        string
		wantInstalled []string
		wantCurrent   string
	}{
		{"Installed_SkipsByDefault", "", "1.2.3", true, "2.0.0", []string{"1.2.3"}, "1.2.3"},
		{"NothingInstalled_Waits", "", "", false, "1.2.3", []string{"1.2.3"}, "1.2.3"},
		{"WaitEnv_ForcesWait", "1", "1.2.3", false, "2.0.0", []string{"1.2.3", "2.0.0"}, "2.0.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(LockWaitEnv, tc.waitEnv)

			p := &fakePkger{name: "claude", latest: tc.preInstalled}
			root := t.TempDir()
			if tc.preInstalled != "" {
				require.NoError(t, Run(pkger.PkgDir{Pkger: p, Root: root}))
			}

			p.latest = tc.latest
			release := holdLock(t, filepath.Join(root, "claude.lock"))
			if tc.runHeld {
				require.NoError(t, Run(pkger.PkgDir{Pkger: p, Root: root}))
				release()
			} else {
				done := make(chan error, 1)
				go func() { done <- Run(pkger.PkgDir{Pkger: p, Root: root}) }()
				release()
				require.NoError(t, <-done)
			}

			assert.Equal(t, tc.wantInstalled, p.installed)
			assert.Equal(t, tc.wantCurrent, readLink(t, filepath.Join(root, "claude", "current")))
		})
	}
}
