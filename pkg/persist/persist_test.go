package persist

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

const testProjectDir = "/Users/me/proj"

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
	return Share{ProjectDir: testProjectDir}
}

func TestShare_Begin_Existing(t *testing.T) {
	s := testShare(t)
	require.NoError(t, os.MkdirAll(s.realDir(), ioutil.Dir))

	dir, clean, err := s.Begin()
	require.NoError(t, err)
	assert.Equal(t, s.realDir(), dir)
	require.NoError(t, clean())
	assert.NoDirExists(t, s.scratch(), "an existing dir binds directly, no scratch seeded")
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
	assert.NoDirExists(t, s.realDir(), "an untouched folder leaves nothing behind")
}

func TestShare_Settle_Changed(t *testing.T) {
	s := testShare(t)

	_, clean, err := s.Begin()
	require.NoError(t, err)
	writeScratch(t, s, "notes/ideas.txt", "keep this")

	require.NoError(t, clean())

	assertSeeded(t, filepath.Join(s.realDir(), "README.md"))
	assertFile(t, filepath.Join(s.realDir(), "notes/ideas.txt"), "keep this")
	assert.NoDirExists(t, s.scratch())
}

func TestShare_Settle_EmptyDirOnly(t *testing.T) {
	s := testShare(t)

	_, clean, err := s.Begin()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(s.scratch(), "notes"), ioutil.Dir))

	require.NoError(t, clean())

	assertSeeded(t, filepath.Join(s.realDir(), "README.md"))
	info, err := os.Stat(filepath.Join(s.realDir(), "notes"))
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "an added empty dir promotes too")
	assert.NoDirExists(t, s.scratch())
}

func TestShare_Begin_Piggyback(t *testing.T) {
	s := testShare(t)

	// a live run seeds the scratch and edits it
	_, clean, err := s.Begin()
	require.NoError(t, err)
	writeScratch(t, s, "notes/ideas.txt", "live run's edit")

	// a second run piggybacks: the same scratch binds, untouched
	dir, piggybackClean, err := s.Begin()
	require.NoError(t, err)
	assert.Equal(t, s.scratch(), dir)
	assertSeeded(t, filepath.Join(dir, "README.md"))
	assertFile(t, filepath.Join(dir, "notes/ideas.txt"), "live run's edit")

	// the piggyback's cleanup leaves the scratch: the live run is still in
	require.NoError(t, piggybackClean())
	assert.DirExists(t, s.scratch())
	assert.NoDirExists(t, s.realDir())

	// the last run out settles
	require.NoError(t, clean())
	assertSeeded(t, filepath.Join(s.realDir(), "README.md"))
	assertFile(t, filepath.Join(s.realDir(), "notes/ideas.txt"), "live run's edit")
	assert.NoDirExists(t, s.scratch())
}

func TestShare_Begin_LatecomerBindsDir(t *testing.T) {
	s := testShare(t)

	// a live run seeds the scratch; the Dir appears mid-session
	_, clean, err := s.Begin()
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(s.realDir(), ioutil.Dir))

	// the latecomer binds the new Dir, not the seeded scratch
	dir, piggybackClean, err := s.Begin()
	require.NoError(t, err)
	assert.Equal(t, s.realDir(), dir)
	require.NoError(t, piggybackClean())
	require.NoError(t, clean())
}

// TestShare_Begin_LatecomerJoinsExistingDir: the first run binds the existing Dir directly,
// no scratch seeded; the latecomer joins the Dir all the same
func TestShare_Begin_LatecomerJoinsExistingDir(t *testing.T) {
	s := testShare(t)
	require.NoError(t, os.MkdirAll(s.realDir(), ioutil.Dir))

	_, clean, err := s.Begin()
	require.NoError(t, err)
	assert.NoDirExists(t, s.scratch(), "an existing dir binds directly, no scratch seeded")

	dir, piggybackClean, err := s.Begin()
	require.NoError(t, err)
	assert.Equal(t, s.realDir(), dir)

	require.NoError(t, piggybackClean())
	require.NoError(t, clean())
	assert.NoDirExists(t, s.scratch())
}

func TestShare_VerifyShared(t *testing.T) {
	s := testShare(t)

	require.Error(t, s.verifyShared(), "nothing present: neither the scratch nor the Dir")

	require.NoError(t, os.MkdirAll(s.realDir(), ioutil.Dir))
	require.NoError(t, s.verifyShared(), "the Dir bound directly is the share")
}

func TestShare_Settle_IntoExistingDir(t *testing.T) {
	s := testShare(t)

	_, clean, err := s.Begin()
	require.NoError(t, err)

	// the run changes the seeded README, adds files; the Dir gains its own copies meanwhile
	writeScratch(t, s, "README.md", "run's edit")
	writeScratch(t, s, "notes/new.txt", "new file")
	writeScratch(t, s, "notes/same.txt", "same")
	require.NoError(t, os.MkdirAll(s.realDir(), ioutil.Dir))
	require.NoError(t, ioutil.SafeWriteFile(filepath.Join(s.realDir(), "README.md"), []byte("dir's edit")))
	require.NoError(t, ioutil.SafeWriteFile(filepath.Join(s.realDir(), "notes/keep.txt"), []byte("keep")))
	require.NoError(t, ioutil.SafeWriteFile(filepath.Join(s.realDir(), "notes/same.txt"), []byte("same")))

	require.NoError(t, clean())

	// the run's copy settles in; the Dir's differing copy is kept aside as a variant
	assertFile(t, filepath.Join(s.realDir(), "README.md"), "run's edit")
	variants, err := filepath.Glob(filepath.Join(s.realDir(), "README.run-*.md"))
	require.NoError(t, err)
	require.Len(t, variants, 1)
	assertFile(t, variants[0], "dir's edit")

	// the run's new file lands in; the identical file leaves one copy; the Dir-only file is untouched
	assertFile(t, filepath.Join(s.realDir(), "notes/new.txt"), "new file")
	assertFile(t, filepath.Join(s.realDir(), "notes/same.txt"), "same")
	sameVariants, err := filepath.Glob(filepath.Join(s.realDir(), "notes/same.run-*"))
	require.NoError(t, err)
	assert.Empty(t, sameVariants, "an identical file promotes without a variant")
	assertFile(t, filepath.Join(s.realDir(), "notes/keep.txt"), "keep")
	assert.NoDirExists(t, s.scratch())
}

func TestShare_Settle_CrashLeftover_IntoExistingDir(t *testing.T) {
	s := testShare(t)

	// a crashed run leaves a changed scratch; the Dir exists from an earlier settle
	crashRun(t)
	writeScratch(t, s, "notes.txt", "written before the crash")
	require.NoError(t, os.MkdirAll(s.realDir(), ioutil.Dir))
	require.NoError(t, os.WriteFile(filepath.Join(s.realDir(), "README.md"), []byte("dir's own"), ioutil.File))

	// the next Begin settles the leftover scratch into the existing Dir
	dir, clean, err := s.Begin()
	require.NoError(t, err)
	assert.Equal(t, s.realDir(), dir)
	require.NoError(t, clean())

	assertFile(t, filepath.Join(s.realDir(), "notes.txt"), "written before the crash")
	variants, err := filepath.Glob(filepath.Join(s.realDir(), "README.run-*.md"))
	require.NoError(t, err)
	require.Len(t, variants, 1)
	assertFile(t, variants[0], "dir's own")
	assertSeeded(t, filepath.Join(s.realDir(), "README.md"))
	assert.NoDirExists(t, s.scratch())
}

func TestShare_Settle_CrashLeftover(t *testing.T) {
	s := testShare(t)

	// a run seeded the scratch, changed it, then crashed before its cleanup ran
	crashRun(t)
	writeScratch(t, s, "notes.txt", "written before the crash")

	// the next Begin settles the leftover scratch
	dir, clean, err := s.Begin()
	require.NoError(t, err)
	assert.Equal(t, s.realDir(), dir)
	require.NoError(t, clean())

	assertSeeded(t, filepath.Join(s.realDir(), "README.md"))
	assertFile(t, filepath.Join(s.realDir(), "notes.txt"), "written before the crash")
	assert.NoDirExists(t, s.scratch())
}

func TestShare_Begin_CrashLeftover_Unchanged(t *testing.T) {
	s := testShare(t)

	// a crashed run leaves an untouched scratch; the Dir exists from an earlier settle
	crashRun(t)
	require.NoError(t, os.MkdirAll(s.realDir(), ioutil.Dir))
	require.NoError(t, ioutil.SafeWriteFile(filepath.Join(s.realDir(), "README.md"), []byte("dir's own")))

	// the next Begin sweeps the unchanged scratch and binds the Dir
	dir, clean, err := s.Begin()
	require.NoError(t, err)
	assert.Equal(t, s.realDir(), dir)
	require.NoError(t, clean())

	assertFile(t, filepath.Join(s.realDir(), "README.md"), "dir's own")
	variants, err := filepath.Glob(filepath.Join(s.realDir(), "*.run-*"))
	require.NoError(t, err)
	assert.Empty(t, variants, "an unchanged leftover promotes nothing")
	assert.NoDirExists(t, s.scratch())
}

// crashRun runs a Share's Begin in a child process that exits without cleanup: the
// kernel releases the crashed run's registry entry, as a real crash does
func crashRun(t *testing.T) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHelperCrashRun$") // #nosec G204,G702 -- re-execs the test binary itself
	cmd.Env = append(os.Environ(), "CCBOX_CRASH_RUN=1")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "child output:\n%s", out)
}

// TestHelperCrashRun is crashRun's child process
func TestHelperCrashRun(t *testing.T) {
	if os.Getenv("CCBOX_CRASH_RUN") != "1" {
		t.Skip("helper: not under test")
	}
	_, _, err := Share{ProjectDir: testProjectDir}.Begin()
	require.NoError(t, err)
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
	got, err := os.ReadFile(path) // #nosec G304 -- reads the test's own path
	require.NoErrorf(t, err, "read %s", path)
	assert.Equal(t, want, string(got), path)
}
