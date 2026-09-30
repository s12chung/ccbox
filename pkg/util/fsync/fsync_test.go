package fsync

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

func srcFS() fstest.MapFS {
	return fstest.MapFS{
		"AGENTS.md":         {Data: []byte("new agents")},
		"settings.json":     {Data: []byte("new settings")},
		"hooks/tripwire.sh": {Data: []byte("new hook")}, // nested → exercises tree + .sh mode
	}
}

func TestSeed_Fresh(t *testing.T) {
	dest := t.TempDir()

	renamed, err := Seed(srcFS(), dest)
	require.NoError(t, err)
	assert.Empty(t, renamed, "fresh seed should back up nothing")

	// Nested file preserved; modes set by extension.
	assertFile(t, filepath.Join(dest, "AGENTS.md"), "new agents")
	assertFile(t, filepath.Join(dest, "settings.json"), "new settings")
	assertFile(t, filepath.Join(dest, "hooks/tripwire.sh"), "new hook")

	assertMode(t, filepath.Join(dest, "hooks/tripwire.sh"), ioutil.ExecFile)
	assertMode(t, filepath.Join(dest, "settings.json"), ioutil.File)
}

func TestSeed_BacksUpExisting(t *testing.T) {
	dest := t.TempDir()
	writeFile(t, filepath.Join(dest, "AGENTS.md"), "old agents")
	writeFile(t, filepath.Join(dest, "settings.json"), "old settings")

	renamed, err := Seed(srcFS(), dest)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{
		filepath.Join(dest, "AGENTS.old.md"),
		filepath.Join(dest, "settings.old.json"),
	}, renamed)

	// Backups hold old contents; new contents are in place.
	assertFile(t, filepath.Join(dest, "AGENTS.old.md"), "old agents")
	assertFile(t, filepath.Join(dest, "settings.old.json"), "old settings")
	assertFile(t, filepath.Join(dest, "AGENTS.md"), "new agents")
	assertFile(t, filepath.Join(dest, "settings.json"), "new settings")

	// The file with no pre-existing dest is written, with no backup.
	assertFile(t, filepath.Join(dest, "hooks/tripwire.sh"), "new hook")
	assertNotExist(t, filepath.Join(dest, "hooks/tripwire.old.sh"))
}

func TestSeed_SkipsIdentical(t *testing.T) {
	dest := t.TempDir()
	// Each dest already holds its source contents.
	writeFile(t, filepath.Join(dest, "AGENTS.md"), "new agents")
	writeFile(t, filepath.Join(dest, "settings.json"), "new settings")
	mkdirAll(t, filepath.Join(dest, "hooks"))
	writeFile(t, filepath.Join(dest, "hooks/tripwire.sh"), "new hook")

	renamed, err := Seed(srcFS(), dest)
	require.ErrorIs(t, err, ErrNoChanges)
	assert.Empty(t, renamed, "identical files should back up nothing")

	// Untouched: contents stay, no backups created.
	assertFile(t, filepath.Join(dest, "AGENTS.md"), "new agents")
	assertFile(t, filepath.Join(dest, "settings.json"), "new settings")
	assertFile(t, filepath.Join(dest, "hooks/tripwire.sh"), "new hook")
	assertNotExist(t, filepath.Join(dest, "AGENTS.old.md"))
	assertNotExist(t, filepath.Join(dest, "settings.old.json"))
}

func TestSeed_PartiallyIdentical(t *testing.T) {
	dest := t.TempDir()
	// One dest matches its source; another differs.
	writeFile(t, filepath.Join(dest, "AGENTS.md"), "new agents")
	writeFile(t, filepath.Join(dest, "settings.json"), "old settings")

	renamed, err := Seed(srcFS(), dest)
	require.NoError(t, err, "a change alongside identical files is not ErrNoChanges")
	assert.Equal(t, []string{filepath.Join(dest, "settings.old.json")}, renamed)

	assertFile(t, filepath.Join(dest, "AGENTS.md"), "new agents")
	assertFile(t, filepath.Join(dest, "settings.json"), "new settings")
}

func TestSeed_BackupExistsErrors(t *testing.T) {
	dest := t.TempDir()
	writeFile(t, filepath.Join(dest, "AGENTS.md"), "old agents")
	writeFile(t, filepath.Join(dest, "AGENTS.old.md"), "stale backup")

	// The live file's backup already exists.
	src := fstest.MapFS{"AGENTS.md": {Data: []byte("new agents")}}
	_, err := Seed(src, dest)
	require.Error(t, err)

	// Neither the live file nor the pre-existing backup was touched.
	assertFile(t, filepath.Join(dest, "AGENTS.md"), "old agents")
	assertFile(t, filepath.Join(dest, "AGENTS.old.md"), "stale backup")
}

func TestMerge_Fresh(t *testing.T) {
	parent := t.TempDir()
	src, dst := filepath.Join(parent, "src"), filepath.Join(parent, "dst") // dst absent
	mkdirAll(t, src)
	writeFile(t, filepath.Join(src, "AGENTS.md"), "new agents")
	mkdirAll(t, filepath.Join(src, "hooks"))
	writeFile(t, filepath.Join(src, "hooks/tripwire.sh"), "new hook")

	asides, err := Merge(src, dst, "run-test")
	require.NoError(t, err)
	assert.Empty(t, asides, "a fresh dst moves nothing aside")

	// the whole src dir moved in
	assertFile(t, filepath.Join(dst, "AGENTS.md"), "new agents")
	assertFile(t, filepath.Join(dst, "hooks/tripwire.sh"), "new hook")
	assert.NoDirExists(t, src)
}

func TestMerge_NewAndIdentical(t *testing.T) {
	parent := t.TempDir()
	src, dst := filepath.Join(parent, "src"), filepath.Join(parent, "dst")
	mkdirAll(t, src)
	writeFile(t, filepath.Join(src, "same.txt"), "same")
	mkdirAll(t, filepath.Join(src, "notes"))
	writeFile(t, filepath.Join(src, "notes/new.txt"), "new")
	mkdirAll(t, dst)
	writeFile(t, filepath.Join(dst, "same.txt"), "same")

	asides, err := Merge(src, dst, "run-test")
	require.NoError(t, err)
	assert.Empty(t, asides)

	// the new file is written in; src keeps everything: its caller drops it wholesale
	assertFile(t, filepath.Join(dst, "notes/new.txt"), "new")
	assertFile(t, filepath.Join(dst, "same.txt"), "same")
	assertFile(t, filepath.Join(src, "same.txt"), "same")
	assertFile(t, filepath.Join(src, "notes/new.txt"), "new")
}

func TestMerge_AllIdentical(t *testing.T) {
	parent := t.TempDir()
	src, dst := filepath.Join(parent, "src"), filepath.Join(parent, "dst")
	mkdirAll(t, src)
	writeFile(t, filepath.Join(src, "same.txt"), "same")
	mkdirAll(t, dst)
	writeFile(t, filepath.Join(dst, "same.txt"), "same")

	// unlike Seed, an unchanged merge is no error: Merge has no ErrNoChanges
	asides, err := Merge(src, dst, "run-test")
	require.NoError(t, err)
	assert.Empty(t, asides)
	assertFile(t, filepath.Join(dst, "same.txt"), "same")
}

func TestMerge_ChangedAsideDst(t *testing.T) {
	parent := t.TempDir()
	src, dst := filepath.Join(parent, "src"), filepath.Join(parent, "dst")
	mkdirAll(t, src)
	writeFile(t, filepath.Join(src, "README.md"), "run's edit")
	mkdirAll(t, filepath.Join(dst, "notes"))
	writeFile(t, filepath.Join(dst, "README.md"), "dir's edit")
	writeFile(t, filepath.Join(dst, "notes/keep.txt"), "keep")

	asides, err := Merge(src, dst, "run-test")
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join(dst, "README.run-test.md")}, asides)

	// the run's copy lands in dst; dst's copy moved aside; the dst-only file is untouched
	assertFile(t, filepath.Join(dst, "README.md"), "run's edit")
	assertFile(t, filepath.Join(dst, "README.run-test.md"), "dir's edit")
	assertFile(t, filepath.Join(dst, "notes/keep.txt"), "keep")
}

func TestFile_Seeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "ccbox.yaml") // parent dir absent

	require.NoError(t, File(path, "new body"))

	assertFile(t, path, "new body")
	assertMode(t, path, ioutil.File)
}

func TestFile_SkipsExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ccbox.yaml")
	writeFile(t, path, "user's own config")

	err := File(path, "new body")
	require.ErrorIs(t, err, ErrExists)
	require.ErrorContains(t, err, path, "the error carries the path")
	assertFile(t, path, "user's own config")
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(body), ioutil.File))
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(path, ioutil.Dir))
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path) // #nosec G304 -- reads the caller-specified fixture path
	require.NoErrorf(t, err, "read %s", path)
	assert.Equal(t, want, string(got), path)
}

func assertNotExist(t *testing.T, path string) {
	t.Helper()
	assert.NoFileExists(t, path)
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, want, info.Mode().Perm(), path)
}
