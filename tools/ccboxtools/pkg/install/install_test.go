package install

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkger"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/flock"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/installutil"
)

// fakePkger serves a canned latest version; Install simulates npm's bin layout.
type fakePkger struct {
	name      string
	latest    string
	latestErr error

	installed []string
}

func (f *fakePkger) Name() string                             { return f.name }
func (f *fakePkger) Latest(_ context.Context) (string, error) { return f.latest, f.latestErr }
func (f *fakePkger) RelBin() string                           { return "bin/" + f.name }
func (f *fakePkger) Install(_ context.Context, dir, version string) error {
	f.installed = append(f.installed, version)
	if err := os.MkdirAll(filepath.Join(dir, "bin"), installutil.DirMode); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "bin", f.name), nil, installutil.DirMode)
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

// holdLock takes an uncontended install lock the way flock.Do does, for the test
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

// clisRoot is the clis root as the image mounts it: <temp>/clis, so the
// container's own bin dir is its sibling, inside the cleaned temp tree.
func clisRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "clis")
	require.NoError(t, os.MkdirAll(root, installutil.DirMode))
	return root
}

// binLink reads the container's own bin link the boot writes for the fake's CLI.
func binLink(t *testing.T, root string) string {
	t.Helper()
	return readLink(t, filepath.Join(filepath.Dir(root), "bin", "claude"))
}

func TestFromPkgInfo_UnknownArtifact(t *testing.T) {
	// unrepresentable via the validated config, so Go-constructed only
	info := pkginfo.PkgInfo{
		Name: "claude",
		ReleaseURL: &pkginfo.ReleaseURL{
			URL:      "https://example.dev",
			JQSchema: &pkginfo.JQSchema{Format: "yaml", Version: ".version", DownloadURL: ".url"},
			Artifact: &pkginfo.Artifact{Type: "rpm", RelBin: "bin/claude"},
		},
	}
	root := t.TempDir()

	err := FromPkgInfo(t.Context(), info, root)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `unknown artifact type "rpm"`)
	assert.NoDirExists(t, filepath.Join(root, "claude")) // nothing installed
}

func TestFromPkgDir_Installs(t *testing.T) {
	p := &fakePkger{name: "claude", latest: "1.2.3"}
	root := clisRoot(t)

	require.NoError(t, FromPkgDir(t.Context(), pkger.PkgDir{Pkger: p, Root: root}))

	// the version dir in place, the container's own bin linked at it
	require.DirExists(t, filepath.Join(root, "claude", "1.2.3"))
	assert.Equal(t,
		filepath.Join(root, "claude", "1.2.3", "bin", "claude"),
		binLink(t, root))
	assert.Equal(t, []string{"1.2.3"}, entryNames(t, filepath.Join(root, "claude")))
	// the install lock is root-level, out of prune's reach
	assert.FileExists(t, filepath.Join(root, "claude.lock"))
	// the volume's own bin dir is never written
	assert.NoDirExists(t, filepath.Join(root, "bin"))
}

func TestFromPkgDir_UpgradesAndPrunes(t *testing.T) {
	p := &fakePkger{name: "claude", latest: "1.2.3"}
	root := clisRoot(t)
	require.NoError(t, FromPkgDir(t.Context(), pkger.PkgDir{Pkger: p, Root: root}))

	p.latest = "2.0.0"
	require.NoError(t, FromPkgDir(t.Context(), pkger.PkgDir{Pkger: p, Root: root}))

	assert.Equal(t, []string{"1.2.3", "2.0.0"}, p.installed)
	assert.Equal(t,
		filepath.Join(root, "claude", "2.0.0", "bin", "claude"),
		binLink(t, root))
	assert.Equal(t, []string{"2.0.0"}, entryNames(t, filepath.Join(root, "claude")))
}

func TestFromPkgDir_UpToDate(t *testing.T) {
	p := &fakePkger{name: "claude", latest: "1.2.3"}
	root := clisRoot(t)
	require.NoError(t, FromPkgDir(t.Context(), pkger.PkgDir{Pkger: p, Root: root}))
	require.NoError(t, FromPkgDir(t.Context(), pkger.PkgDir{Pkger: p, Root: root}))

	assert.Equal(t, []string{"1.2.3"}, p.installed) // no re-install on the second run
}

func TestFromPkgDir_MissingExecutable_Reinstalls(t *testing.T) {
	p := &fakePkger{name: "claude", latest: "1.2.3"}
	root := clisRoot(t)
	require.NoError(t, FromPkgDir(t.Context(), pkger.PkgDir{Pkger: p, Root: root}))

	// the version dir outlives its executable (a wiped dir)
	require.NoError(t, os.Remove(filepath.Join(root, "claude", "1.2.3", "bin", "claude")))
	require.NoError(t, FromPkgDir(t.Context(), pkger.PkgDir{Pkger: p, Root: root}))

	assert.Equal(t, []string{"1.2.3", "1.2.3"}, p.installed)
}

func TestFromPkgDir_ChannelFails_Fails(t *testing.T) {
	// nothing stands in for an unresolved latest — not even an installed version
	p := &fakePkger{name: "claude", latest: "1.2.3"}
	root := clisRoot(t)
	require.NoError(t, FromPkgDir(t.Context(), pkger.PkgDir{Pkger: p, Root: root}))

	p.latest, p.latestErr = "", errors.New("channel down")
	err := FromPkgDir(t.Context(), pkger.PkgDir{Pkger: p, Root: root})

	require.ErrorContains(t, err, "channel down")
}

func TestFromPkgDir_LockHeld_Waits(t *testing.T) {
	p := &fakePkger{name: "claude", latest: "1.2.3"}
	root := clisRoot(t)
	pkgDir := pkger.PkgDir{Pkger: p, Root: root}
	require.NoError(t, FromPkgDir(t.Context(), pkgDir))
	release := holdLock(t, filepath.Join(root, "claude.lock")) // another devbox's install in flight

	p.latest = "2.0.0"
	done := make(chan error, 1)
	go func() { done <- FromPkgDir(t.Context(), pkgDir) }()

	assert.Never(t, func() bool { return len(done) > 0 },
		100*time.Millisecond, 10*time.Millisecond, "FromPkgDir returned while the lock was held")

	release()
	require.NoError(t, <-done)
	assert.Equal(t, []string{"1.2.3", "2.0.0"}, p.installed)
	assert.Equal(t,
		filepath.Join(root, "claude", "2.0.0", "bin", "claude"),
		binLink(t, root))
}

func TestFromPkgDir_Prunes_AroundHeldVersion(t *testing.T) {
	p := &fakePkger{name: "claude", latest: "1.2.3"}
	root := clisRoot(t)
	pkgDir := pkger.PkgDir{Pkger: p, Root: root}
	require.NoError(t, FromPkgDir(t.Context(), pkgDir))

	// another devbox holds 1.2.3 for its life
	hold, err := flock.Hold(pkgDir.VersionDir("1.2.3"))
	require.NoError(t, err)

	p.latest = "2.0.0"
	require.NoError(t, FromPkgDir(t.Context(), pkgDir))
	assert.Equal(t, []string{"1.2.3", "2.0.0"}, entryNames(t, filepath.Join(root, "claude")))

	// released, the next boot's prune takes the version — up to date, no install needed
	require.NoError(t, hold())
	require.NoError(t, FromPkgDir(t.Context(), pkgDir))

	assert.Equal(t, []string{"2.0.0"}, entryNames(t, filepath.Join(root, "claude")))
	assert.Equal(t, []string{"1.2.3", "2.0.0"}, p.installed)
}
