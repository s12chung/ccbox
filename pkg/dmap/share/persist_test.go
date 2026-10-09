package share

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/scratch"
	"github.com/s12chung/ccbox/pkg/util/slug"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

// The tests pin PersistDir's own wiring — the paths it lands on and the seed it binds.
// The driver's behavior is pinned in pkg/util/scratch.

const testProjectDir = "/Users/me/proj"

func TestSeedFS(t *testing.T) {
	f := seedFS()
	assert.NoError(t, fstest.TestFS(f, "README.md"))
}

// testPersistDir builds a PersistDir over a fresh temp home
func testPersistDir(t *testing.T) scratch.Binder {
	t.Helper()
	testutil.Home(t)
	return PersistDir(testProjectDir)
}

// realDir is the project's persistent state dir: ~/.ccbox/persist/<slug>
func realDir() string { return filepath.Join(userdir.Dir(), "persist", slug.Path(testProjectDir)) }

// scratchPath is the seeded scratch copy of realDir: ~/.ccbox/tmp/persist/<slug>
func scratchPath() string { return filepath.Join(userdir.Tmp(), "persist", slug.Path(testProjectDir)) }

// TestPersistDir_RealPath pins Real's path: the project dir's slug under the persist root
func TestPersistDir_RealPath(t *testing.T) {
	testutil.FakeHome(t, "/home/me")
	assert.Equal(t, "/home/me/.ccbox/persist/-Users-me-proj", PersistDir(testProjectDir).RealPath)
}

func TestPersistDir_Begin(t *testing.T) {
	t.Run("Scratch", func(t *testing.T) {
		s := testPersistDir(t)

		bind, clean, err := s.Begin()
		require.NoError(t, err)
		assert.Equal(t, scratchPath(), bind.HostPath, "the tmp dir binds while Real is missing")
		assertSeeded(t, filepath.Join(bind.HostPath, "README.md"))

		// the run's changes promote into realDir
		writeScratch(t, "notes.txt", "run's edit")
		require.NoError(t, clean())
		assertSeeded(t, filepath.Join(realDir(), "README.md"))
		assertFile(t, filepath.Join(realDir(), "notes.txt"), "run's edit")
		assert.NoDirExists(t, scratchPath())
	})

	t.Run("Existing", func(t *testing.T) {
		s := testPersistDir(t)
		require.NoError(t, os.MkdirAll(realDir(), osutil.Dir))

		bind, clean, err := s.Begin()
		require.NoError(t, err)
		assert.Equal(t, realDir(), bind.HostPath, "an existing dir binds directly")
		require.NoError(t, clean())
		assert.NoDirExists(t, scratchPath(), "an existing dir binds directly, no scratch seeded")
	})
}

// writeScratch writes body at rel under the run's scratch
func writeScratch(t *testing.T, rel, body string) {
	t.Helper()
	path := filepath.Join(scratchPath(), rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), osutil.Dir))
	require.NoError(t, os.WriteFile(path, []byte(body), osutil.File))
}

func assertSeeded(t *testing.T, path string) {
	t.Helper()
	seedBody, err := fs.ReadFile(seedFS(), "README.md")
	require.NoError(t, err)
	assertFile(t, path, string(seedBody))
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path) // #nosec G304 -- reads the test's own path
	require.NoErrorf(t, err, "read %s", path)
	assert.Equal(t, want, string(got), path)
}
