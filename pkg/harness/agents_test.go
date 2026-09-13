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

// resetUserDir points userdir at a fresh temp tree with the shared AGENTS docs seeded
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
func claudeAdminMd(userDir string) string {
	return filepath.Join(userDir, "claude", adminMdFileName("CLAUDE.md"))
}
func userAgentsMd(userDir string) string { return filepath.Join(userDir, userAgentsMdFileName) }
func userAgentsAdminMd(userDir string) string {
	return filepath.Join(userDir, adminMdFileName(userAgentsMdFileName))
}

func userAgentsReadmeMd(userDir string) string {
	return filepath.Join(userDir, userAgentsReadmeMdFileName)
}

func claudeScratch(userDir string) string {
	return filepath.Join(userDir, "tmp", "claude", "CLAUDE.md")
}

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
	for _, tc := range []struct {
		caseName string
		setup    func(t *testing.T, userDir string) // laid before the seed
		admin    *string                            // the admin variant's body after the seed; nil when gone
		readme   *string                            // the explainer's body after the seed; nil when gone
	}{
		{
			caseName: "LaysAll",
			admin:    new(""),
			readme:   new(agentsReadmeMd),
		},
		{
			caseName: "SkipsExistingAdmin",
			setup: func(t *testing.T, userDir string) {
				writeFile(t, userAgentsAdminMd(userDir), "kept")
			},
			admin: new("kept"),
		},
		{
			caseName: "UserDocPreventsAdminAndReadme",
			setup: func(t *testing.T, userDir string) {
				writeFile(t, userAgentsMd(userDir), "mine")
			},
		},
	} {
		t.Run(tc.caseName, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			userDir := userdir.Dir()
			if tc.setup != nil {
				tc.setup(t, userDir)
			}

			require.NoError(t, SafeSeedAgentsMd())

			if tc.admin == nil {
				assert.True(t, gone(t, userAgentsAdminMd(userDir)), "the admin variant is not laid")
			} else {
				assert.Equal(t, *tc.admin, readFile(t, userAgentsAdminMd(userDir)))
			}
			if tc.readme == nil {
				assert.True(t, gone(t, userAgentsReadmeMd(userDir)), "the explainer is not laid")
			} else {
				assert.Equal(t, *tc.readme, readFile(t, userAgentsReadmeMd(userDir)))
			}
		})
	}
}

// TestAgentsMdShare_Begin binds the share's source doc, most specific first
func TestAgentsMdShare_Begin(t *testing.T) {
	for _, tc := range []struct {
		caseName string
		setup    func(t *testing.T, userDir string)
		binds    bool   // whether the share binds a scratch
		err      bool   // whether the share errors; no scratch is laid
		body     string // the bound scratch's body; unset when !binds
	}{
		{
			caseName: "ScratchCopy",
			setup: func(t *testing.T, userDir string) {
				writeFile(t, userAgentsMd(userDir), "shared")
			},
			binds: true,
			body:  "shared",
		},
		{
			caseName: "AdminPrefixedScratch",
			setup: func(t *testing.T, userDir string) {
				writeFile(t, userAgentsAdminMd(userDir), "admin rules")
			},
			binds: true,
			body:  adminMd + "admin rules",
		},
		{
			caseName: "SeededDefault",
			binds:    true,
			body:     adminMd,
		},
		{
			caseName: "EmptyUserDocOverridesAdmin",
			setup: func(t *testing.T, userDir string) {
				writeFile(t, userAgentsMd(userDir), "")
				writeFile(t, userAgentsAdminMd(userDir), "admin rules")
			},
			binds: true,
		},
		{
			caseName: "CliAdminPrefixedScratch",
			setup: func(t *testing.T, userDir string) {
				writeFile(t, userAgentsMd(userDir), "shared")
				writeFile(t, claudeAdminMd(userDir), "cli rules")
			},
			binds: true,
			body:  adminMd + "cli rules",
		},
		{
			caseName: "CliFileIsShadowFile",
			setup: func(t *testing.T, userDir string) {
				writeFile(t, claudeCliFile(userDir), shadowBody) // ccbox's shadow file, left by a past run
			},
			binds: true,
			body:  adminMd,
		},
		{
			caseName: "SkipsWhenCliFileExists",
			setup: func(t *testing.T, userDir string) {
				writeFile(t, claudeCliFile(userDir), "cli")
				writeFile(t, claudeAdminMd(userDir), "cli rules")
			},
		},
		{
			caseName: "ErrorsWhenNoSourceDoc",
			setup: func(t *testing.T, userDir string) {
				require.NoError(t, os.Remove(userAgentsAdminMd(userDir)))
			},
			err: true,
		},
	} {
		t.Run(tc.caseName, func(t *testing.T) {
			userDir := resetUserDir(t)
			if tc.setup != nil {
				tc.setup(t, userDir)
			}

			if tc.err {
				shareBindPath, cleanup, err := AgentsMdShare{CLI: MustFor("claude")}.Begin()
				require.Error(t, err)
				assert.Empty(t, shareBindPath)
				assert.True(t, gone(t, claudeScratch(userDir)), "no scratch laid")
				require.NoError(t, cleanup())
				return
			}

			claudeShareBegin(t, func(shareBindPath string) {
				if !tc.binds {
					assert.Empty(t, shareBindPath)
					assert.True(t, gone(t, claudeScratch(userDir)), "no scratch laid")
					return
				}
				assert.Equal(t, claudeScratch(userDir), shareBindPath)
				assert.Equal(t, tc.body, readFile(t, shareBindPath), "the bound scratch's body")
				assert.FileExists(t, claudeCliFile(userDir), "the shadow file is laid")
				assert.Equal(t, shadowBody, readFile(t, claudeCliFile(userDir)), "the shadow file is marked as ccbox's")
			})
		})
	}
}

func TestAgentsMdShare_Begin_EmptyCliFileIsOwned(t *testing.T) {
	userDir := resetUserDir(t)
	writeFile(t, claudeCliFile(userDir), "")

	shareBindPath, cleanup, err := AgentsMdShare{CLI: MustFor("claude")}.Begin()
	require.NoError(t, err)

	assert.Empty(t, shareBindPath, "an empty cliFile is the user's own doc")
	assert.True(t, gone(t, claudeScratch(userDir)), "no scratch laid")

	require.NoError(t, cleanup())
	assert.Empty(t, readFile(t, claudeCliFile(userDir)), "the empty doc is untouched")
}

func TestAgentsMdShare_Begin_SettlesLeftover_PromotesChanged(t *testing.T) {
	userDir := resetUserDir(t)
	writeFile(t, claudeScratch(userDir), "edited") // leftover scratch: edited by the crashed run

	claudeShareBegin(t, func(shareBindPath string) {
		assert.Empty(t, shareBindPath) // leftover promoted, do nothing
	})

	// check promotion
	assert.Equal(t, "edited", readFile(t, claudeCliFile(userDir)))
	assert.True(t, gone(t, claudeScratch(userDir)))
}

func TestAgentsMdShare_Begin_SettlesLeftover_ResharesUnchanged(t *testing.T) {
	userDir := resetUserDir(t)

	// leftover scratch: unchanged from its source doc
	writeFile(t, userAgentsMd(userDir), "shared")
	writeFile(t, claudeScratch(userDir), "shared")

	// during session: re-bound as a fresh scratch
	claudeShareBegin(t, func(shareBindPath string) {
		assert.Equal(t, claudeScratch(userDir), shareBindPath)
		assert.Equal(t, "shared", readFile(t, shareBindPath))
		assert.Equal(t, shadowBody, readFile(t, claudeCliFile(userDir)), "the shadow file is laid")
	})

	assert.True(t, gone(t, claudeCliFile(userDir))) // shadow dropped
	assert.True(t, gone(t, claudeScratch(userDir))) // scratch gone
}

// TestAgentsMdShare_Cleanup settles the bound scratch into the cliFile: a diff is promoted
// as-is, and the doc it was bound from is left untouched
func TestAgentsMdShare_Cleanup(t *testing.T) {
	for _, tc := range []struct {
		caseName           string
		setup              func(t *testing.T, userDir string) (path, body string) // the doc kept untouched
		scratchChange      string                                                 // written to the bound scratch during the run; empty skips the edit
		expectedClaudeFile *string                                                // the expected cliFile body after the cleanup
	}{
		{
			caseName: "PromotesChanged",
			setup: func(t *testing.T, userDir string) (string, string) {
				writeFile(t, userAgentsMd(userDir), "shared")
				return userAgentsMd(userDir), "shared"
			},
			scratchChange:      "memory",
			expectedClaudeFile: new("memory"),
		},
		{
			caseName: "PromotesAdminScratchWithPrefix",
			setup: func(t *testing.T, userDir string) (string, string) {
				writeFile(t, userAgentsAdminMd(userDir), "admin rules")
				return userAgentsAdminMd(userDir), "admin rules"
			},
			scratchChange:      adminMd + "admin rules, edited",
			expectedClaudeFile: new(adminMd + "admin rules, edited"),
		},
		{
			caseName: "PromotesCLIAdminScratchWithPrefix",
			setup: func(t *testing.T, userDir string) (string, string) {
				writeFile(t, claudeAdminMd(userDir), "admin claude rules")
				return claudeAdminMd(userDir), "admin claude rules"
			},
			scratchChange:      adminMd + "admin claude rules, edited",
			expectedClaudeFile: new(adminMd + "admin claude rules, edited"),
		},
		{
			caseName: "KeepsOriginalWhenUnchanged",
			setup: func(t *testing.T, userDir string) (string, string) {
				writeFile(t, userAgentsMd(userDir), "shared")
				return userAgentsMd(userDir), "shared"
			},
		},
		{
			caseName: "NoopWhenNotBound",
			setup: func(t *testing.T, userDir string) (string, string) {
				writeFile(t, claudeCliFile(userDir), "cli")
				return claudeCliFile(userDir), "cli"
			},
			expectedClaudeFile: new("cli"),
		},
	} {
		t.Run(tc.caseName, func(t *testing.T) {
			userDir := resetUserDir(t)
			protectedPath, protectedBody := tc.setup(t, userDir)

			claudeShareBegin(t, func(shareBindPath string) {
				if tc.scratchChange != "" {
					writeFile(t, shareBindPath, tc.scratchChange)
				}
			})

			if tc.expectedClaudeFile == nil {
				assert.True(t, gone(t, claudeCliFile(userDir)), "nothing promoted; the laid shadow file is dropped")
			} else {
				assert.Equal(t, *tc.expectedClaudeFile, readFile(t, claudeCliFile(userDir)))
			}
			assert.True(t, gone(t, claudeScratch(userDir)), "scratch removed")
			assert.Equal(t, protectedBody, readFile(t, protectedPath), "the source doc is untouched")
		})
	}
}

func TestAgentsMdShare_Cleanup_LoneShadow(t *testing.T) {
	userDir := resetUserDir(t)
	writeFile(t, claudeCliFile(userDir), shadowBody) // leftover shadow: its scratch is already gone

	require.NoError(t, AgentsMdShare{CLI: MustFor("claude")}.clean())

	assert.True(t, gone(t, claudeCliFile(userDir)), "the lone shadow is dropped")
}

func TestAgentsMdShare_Cleanup_DiffErrorDropsShadow(t *testing.T) {
	userDir := resetUserDir(t)
	writeFile(t, claudeCliFile(userDir), shadowBody)
	writeFile(t, claudeScratch(userDir), "leftover")
	require.NoError(t, os.Remove(userAgentsAdminMd(userDir))) // no source doc: the diff errors

	require.Error(t, AgentsMdShare{CLI: MustFor("claude")}.clean())

	assert.True(t, gone(t, claudeCliFile(userDir)), "the shadow is dropped despite the failed settle")
	assert.False(t, gone(t, claudeScratch(userDir)), "the scratch is kept for the next run")
}

func TestAgentsMdShare_Cleanup_KeepsScratchOnPromoteError(t *testing.T) {
	userDir := resetUserDir(t)
	shareBindPath, cleanup, err := AgentsMdShare{CLI: MustFor("claude")}.Begin()
	require.NoError(t, err)
	writeFile(t, shareBindPath, "memory")
	require.NoError(t, os.Remove(claudeCliFile(userDir)))               // clear the laid shadow file
	require.NoError(t, os.MkdirAll(claudeCliFile(userDir), ioutil.Dir)) // a directory is unreadable: promotion fails

	require.Error(t, cleanup(), "cliFile is a directory")
	assert.Equal(t, "memory", readFile(t, shareBindPath), "scratch kept for the next run")
}
