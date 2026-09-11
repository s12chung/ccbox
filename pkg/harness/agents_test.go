package harness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

// The tests pin AgentsMdShare on claude's paths under a temp userDir.

// resetUserDir points userdir at a fresh temp tree with the shared AGENTS doc seeded
func resetUserDir(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	require.NoError(t, SafeSeedAgentsMd())
	return userdir.Dir()
}

// claudeShareBegin begins a bound share for claude, runs body with the bound scratch, then
// cleans up
func claudeShareBegin(t *testing.T, body func(shareBindPath string)) {
	t.Helper()
	shareBindPath, cleanup, err := AgentsMdShare{CLI: MustFor("claude")}.Begin()
	require.NoError(t, err)
	if body != nil {
		body(shareBindPath)
	}
	require.NoError(t, cleanup())
}

func claudeCliFile(userDir string) string { return filepath.Join(userDir, "claude", "CLAUDE.md") }
func claudeScratch(userDir string) string {
	return filepath.Join(userDir, "tmp", "claude", "CLAUDE.md")
}
func userAgentsMd(userDir string) string { return filepath.Join(userDir, userAgentsMdFileName) }

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), ioutil.Dir))
	require.NoError(t, os.WriteFile(path, []byte(body), ioutil.File))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path) // #nosec G304 -- the test's own path
	require.NoError(t, err)
	return string(body)
}

func gone(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

func TestSafeSeedAgentsMd(t *testing.T) {
	userDir := resetUserDir(t)
	assert.Empty(t, readFile(t, userAgentsMd(userDir)), "laid empty")
}

func TestSafeSeedAgentsMd_SkipsExisting(t *testing.T) {
	userDir := resetUserDir(t)
	writeFile(t, userAgentsMd(userDir), "kept")

	require.NoError(t, SafeSeedAgentsMd())

	assert.Equal(t, "kept", readFile(t, userAgentsMd(userDir)))
}

func TestAgentsMdShare_Begin_BindsScratchCopy(t *testing.T) {
	userDir := resetUserDir(t)
	writeFile(t, userAgentsMd(userDir), "shared")

	claudeShareBegin(t, func(shareBindPath string) {
		assert.Equal(t, claudeScratch(userDir), shareBindPath)
		assert.Equal(t, "shared", readFile(t, shareBindPath), "scratch is a copy of the original")
	})
}

func TestAgentsMdShare_Begin_SkipsWhenCliFileExists(t *testing.T) {
	userDir := resetUserDir(t)
	writeFile(t, claudeCliFile(userDir), "cli")

	claudeShareBegin(t, func(shareBindPath string) {
		assert.Empty(t, shareBindPath, "the cliFile wins")
		assert.True(t, gone(t, claudeScratch(userDir)), "no scratch laid")
	})
}

func TestAgentsMdShare_Begin_PromotesChangedLeftover(t *testing.T) {
	userDir := resetUserDir(t)
	writeFile(t, claudeScratch(userDir), "edited") // left by a crashed run

	claudeShareBegin(t, func(shareBindPath string) {
		assert.Equal(t, "edited", readFile(t, claudeCliFile(userDir)))
		assert.Empty(t, shareBindPath, "the promoted copy wins over a fresh bind")
		assert.True(t, gone(t, claudeScratch(userDir)))
	})
}

func TestAgentsMdShare_Begin_DropsUnchangedLeftover(t *testing.T) {
	userDir := resetUserDir(t)
	writeFile(t, userAgentsMd(userDir), "shared") // what the leftover was copied from
	writeFile(t, claudeScratch(userDir), "shared")

	claudeShareBegin(t, func(shareBindPath string) {
		assert.Empty(t, shareBindPath, "leftover run skips the bind")
		assert.True(t, gone(t, claudeScratch(userDir)))
		assert.True(t, gone(t, claudeCliFile(userDir)), "nothing promoted")
	})
}

func TestAgentsMdShare_Cleanup_PromotesChanged(t *testing.T) {
	userDir := resetUserDir(t)
	writeFile(t, userAgentsMd(userDir), "shared")
	claudeShareBegin(t, func(shareBindPath string) {
		writeFile(t, shareBindPath, "memory")
	})

	assert.Equal(t, "memory", readFile(t, claudeCliFile(userDir)))
	assert.Equal(t, "shared", readFile(t, userAgentsMd(userDir)), "original untouched")
	assert.True(t, gone(t, claudeScratch(userDir)))
}

func TestAgentsMdShare_Cleanup_KeepsOriginalWhenUnchanged(t *testing.T) {
	userDir := resetUserDir(t)
	writeFile(t, userAgentsMd(userDir), "shared")
	claudeShareBegin(t, nil)

	assert.True(t, gone(t, claudeCliFile(userDir)), "nothing promoted")
	assert.True(t, gone(t, claudeScratch(userDir)), "scratch removed")
	assert.Equal(t, "shared", readFile(t, userAgentsMd(userDir)), "original untouched")
}

func TestAgentsMdShare_Cleanup_NoopWhenNotBound(t *testing.T) {
	userDir := resetUserDir(t)
	writeFile(t, claudeCliFile(userDir), "cli")
	claudeShareBegin(t, nil)

	assert.Equal(t, "cli", readFile(t, claudeCliFile(userDir)), "the cliFile is untouched")
	assert.True(t, gone(t, claudeScratch(userDir)), "no scratch laid")
}

func TestAgentsMdShare_Cleanup_KeepsScratchOnPromoteError(t *testing.T) {
	userDir := resetUserDir(t)
	shareBindPath, cleanup, err := AgentsMdShare{CLI: MustFor("claude")}.Begin()
	require.NoError(t, err)
	writeFile(t, shareBindPath, "memory")
	require.NoError(t, os.MkdirAll(claudeCliFile(userDir), ioutil.Dir)) // blocks the promotion

	require.Error(t, cleanup(), "cliFile is a directory")
	assert.Equal(t, "memory", readFile(t, shareBindPath), "scratch kept for the next run")
}
