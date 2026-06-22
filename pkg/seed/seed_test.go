package seed

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/perm"
)

func srcFS() fstest.MapFS {
	return fstest.MapFS{
		"CLAUDE.user.md":    {Data: []byte("new claude")},
		"settings.json":     {Data: []byte("new settings")},
		"hooks/tripwire.sh": {Data: []byte("new hook")}, // nested → exercises tree + .sh mode
	}
}

func TestSeedClaudeConfigFresh(t *testing.T) {
	dest := t.TempDir()

	renamed, err := SeedClaudeConfig(srcFS(), dest)
	require.NoError(t, err)
	assert.Empty(t, renamed, "fresh seed should back up nothing")

	// CLAUDE.user.md lands as CLAUDE.md; nested file preserved.
	assertFile(t, filepath.Join(dest, "CLAUDE.md"), "new claude")
	assertFile(t, filepath.Join(dest, "settings.json"), "new settings")
	assertFile(t, filepath.Join(dest, "hooks/tripwire.sh"), "new hook")

	assertNotExist(t, filepath.Join(dest, "CLAUDE.user.md"))
	assertMode(t, filepath.Join(dest, "hooks/tripwire.sh"), perm.ExecFile)
	assertMode(t, filepath.Join(dest, "settings.json"), perm.File)
}

func TestSeedClaudeConfigBacksUpExisting(t *testing.T) {
	dest := t.TempDir()
	writeFile(t, filepath.Join(dest, "CLAUDE.md"), "old claude")
	writeFile(t, filepath.Join(dest, "settings.json"), "old settings")

	renamed, err := SeedClaudeConfig(srcFS(), dest)
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

func TestSeedClaudeConfigBackupExistsErrors(t *testing.T) {
	dest := t.TempDir()
	writeFile(t, filepath.Join(dest, "CLAUDE.md"), "old claude")
	writeFile(t, filepath.Join(dest, "CLAUDE.old.md"), "stale backup")

	// CLAUDE.user.md → CLAUDE.md collides with the live file, whose backup already exists.
	src := fstest.MapFS{"CLAUDE.user.md": {Data: []byte("new claude")}}
	_, err := SeedClaudeConfig(src, dest)
	require.Error(t, err)

	// Neither the live file nor the pre-existing backup was touched.
	assertFile(t, filepath.Join(dest, "CLAUDE.md"), "old claude")
	assertFile(t, filepath.Join(dest, "CLAUDE.old.md"), "stale backup")
}

func TestSeedProject(t *testing.T) {
	dest := t.TempDir()
	src := fstest.MapFS{
		"lessons.md":     {Data: []byte("lessons")},
		"hooks/setup.sh": {Data: []byte("hook")}, // nested → exercises tree + .sh mode
	}

	renamed, err := SeedProject(src, dest)
	require.NoError(t, err)
	assert.Empty(t, renamed)

	// Project files keep their names (no CLAUDE remap), with correct modes.
	assertFile(t, filepath.Join(dest, "lessons.md"), "lessons")
	assertFile(t, filepath.Join(dest, "hooks/setup.sh"), "hook")
	assertMode(t, filepath.Join(dest, "lessons.md"), perm.File)
	assertMode(t, filepath.Join(dest, "hooks/setup.sh"), perm.ExecFile)
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
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
