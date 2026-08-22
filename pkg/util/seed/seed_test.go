package seed

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/perm"
)

func srcFS() fstest.MapFS {
	return fstest.MapFS{
		"AGENTS.user.md":    {Data: []byte("new claude")},
		"settings.json":     {Data: []byte("new settings")},
		"hooks/tripwire.sh": {Data: []byte("new hook")}, // nested → exercises tree + .sh mode
	}
}

// claudeRenames is the shared AGENTS.md into Claude's live file, inlined to keep this pkg harness-free.
var claudeRenames = map[string]string{"AGENTS.user.md": "CLAUDE.md"}

func TestTreeFresh(t *testing.T) {
	dest := t.TempDir()

	renamed, err := Tree(srcFS(), dest, claudeRenames)
	require.NoError(t, err)
	assert.Empty(t, renamed, "fresh seed should back up nothing")

	// AGENTS.user.md lands as CLAUDE.md; nested file preserved.
	assertFile(t, filepath.Join(dest, "CLAUDE.md"), "new claude")
	assertFile(t, filepath.Join(dest, "settings.json"), "new settings")
	assertFile(t, filepath.Join(dest, "hooks/tripwire.sh"), "new hook")

	assertNotExist(t, filepath.Join(dest, "AGENTS.user.md"))
	assertMode(t, filepath.Join(dest, "hooks/tripwire.sh"), perm.ExecFile)
	assertMode(t, filepath.Join(dest, "settings.json"), perm.File)
}

func TestTreeBacksUpExisting(t *testing.T) {
	dest := t.TempDir()
	writeFile(t, filepath.Join(dest, "CLAUDE.md"), "old claude")
	writeFile(t, filepath.Join(dest, "settings.json"), "old settings")

	renamed, err := Tree(srcFS(), dest, claudeRenames)
	require.NoError(t, err)

	assert.ElementsMatch(t, []string{
		filepath.Join(dest, "CLAUDE.old.md"),
		filepath.Join(dest, "settings.old.json"),
	}, renamed)

	// Backups hold old contents; new contents are in place.
	assertFile(t, filepath.Join(dest, "CLAUDE.old.md"), "old claude")
	assertFile(t, filepath.Join(dest, "settings.old.json"), "old settings")
	assertFile(t, filepath.Join(dest, "CLAUDE.md"), "new claude")
	assertFile(t, filepath.Join(dest, "settings.json"), "new settings")

	// The file with no pre-existing dest is written, with no backup.
	assertFile(t, filepath.Join(dest, "hooks/tripwire.sh"), "new hook")
	assertNotExist(t, filepath.Join(dest, "hooks/tripwire.old.sh"))
}

func TestTreeSkipsIdentical(t *testing.T) {
	dest := t.TempDir()
	// Each dest already holds the source contents (CLAUDE.md is the renamed AGENTS.user.md).
	writeFile(t, filepath.Join(dest, "CLAUDE.md"), "new claude")
	writeFile(t, filepath.Join(dest, "settings.json"), "new settings")
	mkdirAll(t, filepath.Join(dest, "hooks"))
	writeFile(t, filepath.Join(dest, "hooks/tripwire.sh"), "new hook")

	renamed, err := Tree(srcFS(), dest, claudeRenames)
	require.ErrorIs(t, err, ErrNoChanges)
	assert.Empty(t, renamed, "identical files should back up nothing")

	// Untouched: contents stay, no backups created.
	assertFile(t, filepath.Join(dest, "CLAUDE.md"), "new claude")
	assertFile(t, filepath.Join(dest, "settings.json"), "new settings")
	assertFile(t, filepath.Join(dest, "hooks/tripwire.sh"), "new hook")
	assertNotExist(t, filepath.Join(dest, "CLAUDE.old.md"))
	assertNotExist(t, filepath.Join(dest, "settings.old.json"))
}

func TestTreePartiallyIdentical(t *testing.T) {
	dest := t.TempDir()
	// One dest matches its source; another differs.
	writeFile(t, filepath.Join(dest, "CLAUDE.md"), "new claude")
	writeFile(t, filepath.Join(dest, "settings.json"), "old settings")

	renamed, err := Tree(srcFS(), dest, claudeRenames)
	require.NoError(t, err, "a change alongside identical files is not ErrNoChanges")
	assert.Equal(t, []string{filepath.Join(dest, "settings.old.json")}, renamed)

	assertFile(t, filepath.Join(dest, "CLAUDE.md"), "new claude")
	assertFile(t, filepath.Join(dest, "settings.json"), "new settings")
}

func TestTreeBackupExistsErrors(t *testing.T) {
	dest := t.TempDir()
	writeFile(t, filepath.Join(dest, "CLAUDE.md"), "old claude")
	writeFile(t, filepath.Join(dest, "CLAUDE.old.md"), "stale backup")

	// AGENTS.user.md → CLAUDE.md collides with the live file, whose backup already exists.
	src := fstest.MapFS{"AGENTS.user.md": {Data: []byte("new claude")}}
	_, err := Tree(src, dest, claudeRenames)
	require.Error(t, err)

	// Neither the live file nor the pre-existing backup was touched.
	assertFile(t, filepath.Join(dest, "CLAUDE.md"), "old claude")
	assertFile(t, filepath.Join(dest, "CLAUDE.old.md"), "stale backup")
}

func TestTreeCodexRenames(t *testing.T) {
	dest := t.TempDir()
	src := fstest.MapFS{
		"AGENTS.user.md": {Data: []byte("new agents")},
		"config.toml":    {Data: []byte("new config")},
	}
	renames := map[string]string{"AGENTS.user.md": "AGENTS.md"}

	renamed, err := Tree(src, dest, renames)
	require.NoError(t, err)
	assert.Empty(t, renamed, "fresh seed should back up nothing")

	// AGENTS.user.md lands as AGENTS.md; config.toml is a regular file.
	assertFile(t, filepath.Join(dest, "AGENTS.md"), "new agents")
	assertFile(t, filepath.Join(dest, "config.toml"), "new config")
	assertNotExist(t, filepath.Join(dest, "AGENTS.user.md"))
	assertMode(t, filepath.Join(dest, "config.toml"), perm.File)
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(body), perm.File))
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(path, perm.Dir))
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
