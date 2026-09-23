package persist

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

func TestSeedFS(t *testing.T) {
	f := SeedFS()
	assert.NoError(t, fstest.TestFS(f, "README.md"))
}

func TestDir(t *testing.T) {
	t.Setenv("HOME", "/home/me")
	assert.Equal(t, "/home/me/.ccbox/persist/-Users-me-proj", Dir("/Users/me/proj"))
}

// testShare builds a Share over a fresh temp home
func testShare(t *testing.T) Share {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	return Share{ProjectDir: "/Users/me/proj"}
}

func TestShare_Begin_Existing(t *testing.T) {
	s := testShare(t)
	require.NoError(t, os.MkdirAll(s.dir(), ioutil.Dir))

	dir, clean, err := s.Begin()
	require.NoError(t, err)
	assert.Equal(t, s.dir(), dir)
	require.NoError(t, clean())
	assert.NoDirExists(t, s.scratch(), "an existing dir binds directly, no scratch laid")
}

func TestShare_Begin_Scratch(t *testing.T) {
	s := testShare(t)

	dir, clean, err := s.Begin()
	require.NoError(t, err)
	assert.Equal(t, s.scratch(), dir)
	assertSeeded(t, filepath.Join(dir, "README.md"))

	require.NoError(t, clean())
	assert.NoDirExists(t, s.scratch())
}

func TestShare_Settle_Unchanged(t *testing.T) {
	s := testShare(t)

	_, clean, err := s.Begin()
	require.NoError(t, err)
	require.NoError(t, clean())

	assert.NoDirExists(t, s.scratch())
	assert.NoDirExists(t, s.dir(), "an untouched folder leaves nothing behind")
}

func TestShare_Settle_Changed(t *testing.T) {
	s := testShare(t)

	_, clean, err := s.Begin()
	require.NoError(t, err)
	writeScratch(t, s, "notes/ideas.txt", "keep this")

	require.NoError(t, clean())

	assertSeeded(t, filepath.Join(s.dir(), "README.md"))
	assertFile(t, filepath.Join(s.dir(), "notes/ideas.txt"), "keep this")
	assert.NoDirExists(t, s.scratch())
}

func TestShare_Settle_EmptyDirOnly(t *testing.T) {
	s := testShare(t)

	_, clean, err := s.Begin()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(s.scratch(), "notes"), ioutil.Dir))

	require.NoError(t, clean())

	assertSeeded(t, filepath.Join(s.dir(), "README.md"))
	info, err := os.Stat(filepath.Join(s.dir(), "notes"))
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "an added empty dir promotes too")
	assert.NoDirExists(t, s.scratch())
}

func TestShare_Settle_CrashLeftover(t *testing.T) {
	s := testShare(t)

	// a run seeded the scratch, changed it, then crashed before its cleanup ran
	_, _, err := s.Begin() // the crash: its clean never runs
	require.NoError(t, err)
	writeScratch(t, s, "notes.txt", "written before the crash")

	// the next Begin settles the leftover scratch
	dir, clean, err := s.Begin()
	require.NoError(t, err)
	assert.Equal(t, s.dir(), dir)
	require.NoError(t, clean())

	assertSeeded(t, filepath.Join(s.dir(), "README.md"))
	assertFile(t, filepath.Join(s.dir(), "notes.txt"), "written before the crash")
	assert.NoDirExists(t, s.scratch())
}

// writeScratch writes body at rel under the run's scratch
func writeScratch(t *testing.T, s Share, rel, body string) {
	t.Helper()
	path := filepath.Join(s.scratch(), rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), ioutil.Dir))
	require.NoError(t, os.WriteFile(path, []byte(body), ioutil.File))
}

func assertSeeded(t *testing.T, path string) {
	t.Helper()
	seedBody, err := fs.ReadFile(SeedFS(), "README.md")
	require.NoError(t, err)
	assertFile(t, path, string(seedBody))
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path) // #nosec G304 -- reads the test's own laid path
	require.NoErrorf(t, err, "read %s", path)
	assert.Equal(t, want, string(got), path)
}
