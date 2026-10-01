package clitmpl

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

// resetUserConfigDir redirects the user clis tree to a fresh temp dir, restoring
// the swapped global afterwards; it returns the temp tree's root.
func resetUserConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	saved := userConfigDir
	t.Cleanup(func() { userConfigDir = saved })
	userConfigDir = dir
	return dir
}

// writeCli writes a cli at dir/clis/<name> like the clis tree: a CLI.yaml body
// plus optional config files under config/.
func writeCli(t *testing.T, dir, name, body string, configFiles map[string]string) {
	t.Helper()
	root := filepath.Join(dir, "clis", name)
	require.NoError(t, os.MkdirAll(root, ioutil.Dir))
	require.NoError(t, os.WriteFile(filepath.Join(root, "CLI.yaml"), []byte(body), ioutil.File))
	for p, content := range configFiles {
		dest := filepath.Join(root, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(dest), ioutil.Dir))
		require.NoError(t, os.WriteFile(dest, []byte(content), ioutil.File))
	}
}

// dummyCLI records what Load hands parse, standing in for cli.CLI.
type dummyCLI struct {
	name          string
	isUserDefined bool
}

// parseDummy is the parse test double: it rejects the body "bad\n" like a
// CLI.yaml failing validation.
func parseDummy(name string, body []byte, isUserDefined bool) (dummyCLI, error) {
	if string(body) == "bad\n" {
		return dummyCLI{}, errors.New("dummy: bad cli")
	}
	return dummyCLI{name: name, isUserDefined: isUserDefined}, nil
}

// embedCliNames lists the embedded clis' names, via Load's own walk.
func embedCliNames(t *testing.T) []string {
	t.Helper()
	clis, warns, err := Load(EmbedTree(), parseDummy)
	require.NoError(t, err)
	require.Empty(t, warns)
	names := make([]string, 0, len(clis))
	for _, c := range clis {
		names = append(names, c.name)
	}
	return names
}

func TestLoad_Walk(t *testing.T) {
	t.Run("user tree warns per bad and stray dir, ignoring stray files", func(t *testing.T) {
		dir := resetUserConfigDir(t)
		writeCli(t, dir, "bad", "bad\n", nil)
		writeCli(t, dir, "good", "ok\n", nil)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "clis", "stray"), ioutil.Dir)) // stray dir
		require.NoError(t, os.WriteFile(filepath.Join(dir, "clis", "strayfile"), []byte("junk"), ioutil.File))

		clis, warns, err := Load(UserTree(), parseDummy)

		require.NoError(t, err)
		require.Len(t, clis, 1)
		assert.True(t, clis[0].isUserDefined)
		require.Len(t, warns, 2) // glob order; strayfile is ignored
		require.ErrorContains(t, warns[0], "bad: ")
		require.ErrorContains(t, warns[0], "dummy: bad cli")
		require.ErrorContains(t, warns[1], "stray: ")
		require.ErrorContains(t, warns[1], "not found")
	})

	t.Run("user tree may be empty", func(t *testing.T) {
		resetUserConfigDir(t)

		clis, warns, err := Load(UserTree(), parseDummy)

		require.NoError(t, err)
		assert.Empty(t, clis)
		assert.Empty(t, warns)
	})

	t.Run("embed tree loads in name order, isUserDefined false", func(t *testing.T) {
		clis, warns, err := Load(EmbedTree(), parseDummy)

		require.NoError(t, err)
		assert.Empty(t, warns)
		names := make([]string, 0, len(clis))
		for _, c := range clis {
			names = append(names, c.name)
			assert.False(t, c.isUserDefined)
		}
		assert.Equal(t, []string{"claude", "codex", "grok", "opencode", "pi"}, names)
	})

	t.Run("embed tree cli errors are fatal", func(t *testing.T) {
		dir := t.TempDir()
		writeCli(t, dir, "bad", "bad\n", map[string]string{"config/x": "junk"}) // config exists, so parse is reached
		tree := Tree{fsys: os.DirFS(dir)}                                       // an embed-like tree: isUserDefined false

		clis, warns, err := Load(tree, parseDummy)

		require.Error(t, err)
		assert.Nil(t, clis)
		assert.Empty(t, warns)
		require.ErrorContains(t, err, "bad: ")
		require.ErrorContains(t, err, "dummy: bad cli")
	})

	t.Run("embed tree empty is fatal", func(t *testing.T) {
		dir := t.TempDir()
		tree := Tree{fsys: os.DirFS(dir)} // no clis/ at all

		_, _, err := Load(tree, parseDummy)

		require.Error(t, err)
		assert.ErrorContains(t, err, "no clis/* found")
	})
}

func TestLoad_ValidateConfigDir(t *testing.T) {
	t.Run("user tree config may be absent", func(t *testing.T) {
		dir := resetUserConfigDir(t)
		writeCli(t, dir, "mycli", "ok\n", nil)

		clis, warns, err := Load(UserTree(), parseDummy)

		require.NoError(t, err)
		assert.Empty(t, warns)
		require.Len(t, clis, 1)
	})

	t.Run("user tree config as a file warns", func(t *testing.T) {
		dir := resetUserConfigDir(t)
		writeCli(t, dir, "filecfg", "ok\n", map[string]string{"config": "junk"}) // seed config is a file

		clis, warns, err := Load(UserTree(), parseDummy)

		require.NoError(t, err)
		assert.Empty(t, clis)
		require.Len(t, warns, 1)
		require.ErrorContains(t, warns[0], "filecfg: ")
		require.ErrorContains(t, warns[0], "not a directory")
	})

	t.Run("embed tree config must exist", func(t *testing.T) {
		dir := t.TempDir()
		writeCli(t, dir, "bare", "ok\n", nil)
		tree := Tree{fsys: os.DirFS(dir)} // an embed-like tree: isUserDefined false

		_, _, err := Load(tree, parseDummy)

		require.Error(t, err)
		require.ErrorContains(t, err, "bare: ")
		assert.ErrorContains(t, err, "clis/bare/config")
	})
}

func TestUserCLIConfigSeedFS(t *testing.T) {
	t.Run("embed tree seeds each cli's own config", func(t *testing.T) {
		for _, name := range embedCliNames(t) {
			fsys := UserCLIConfigSeedFS(name, false)
			assert.NotContainsf(t, seedPaths(t, fsys), "AGENTS.md",
				"shared AGENTS doc is bind-mounted at runtime, never seeded: %s", name)
		}

		claudeFS := UserCLIConfigSeedFS("claude", false) // per-CLI tree rooted at its config dir
		want := []string{"hooks/secret-tripwire.sh", "settings.json", "statusline.sh"}
		assert.Equal(t, want, seedPaths(t, claudeFS))
	})

	t.Run("user tree seeds the user's config only", func(t *testing.T) {
		dir := resetUserConfigDir(t)
		writeCli(t, dir, "mycli", "ok\n", map[string]string{"config/settings.toml": "[x]\n"})

		fsys := UserCLIConfigSeedFS("mycli", true)

		assert.Equal(t, []string{"settings.toml"}, seedPaths(t, fsys), "user config tree only")
	})

	t.Run("user tree without config seeds an empty one", func(t *testing.T) {
		dir := resetUserConfigDir(t)
		writeCli(t, dir, "bare", "ok\n", nil)

		bareFS := UserCLIConfigSeedFS("bare", true)

		assert.Empty(t, seedPaths(t, bareFS), "no config tree seeded: UserCLIConfigSeedFS mkdirs an empty one")
	})
}

func seedPaths(t *testing.T, fsys fs.FS) []string {
	t.Helper()
	var paths []string
	require.NoError(t, fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		require.NoError(t, err)
		if !d.IsDir() {
			paths = append(paths, p)
		}
		return nil
	}))
	slices.Sort(paths)
	return paths
}
