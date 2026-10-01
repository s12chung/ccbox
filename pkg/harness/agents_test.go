package harness

import (
	"os"
	"os/exec"
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

// claudeShare is the share under test
func claudeShare() AgentsMdShare {
	return AgentsMdShare{CLI: MustFor("claude"), ScratchMount: "/home/ccbox/.ccbox/tmp/claude/AGENTS.md"}
}

// claudeShareBegin begins a bound share for claude, runs body with the bound scratch dir,
// then cleans up
func claudeShareBegin(t *testing.T, body func(scratchFile string)) {
	t.Helper()
	scratchFile, cleanup, err := claudeShare().Begin()
	require.NoError(t, err)
	if body != nil {
		body(scratchFile)
	}
	require.NoError(t, cleanup())
}

// claudeTarget is the realCliFile symlink's target
func claudeTarget() string { return claudeShare().cliFileTarget() }

func claudeCliFile(userDir string) string { return filepath.Join(userDir, "claude", AgentsMdFileName) }

func claudeAdminMd(userDir string) string {
	return filepath.Join(userDir, "claude", agentsAdminMdFileName)
}
func userAgentsMd(userDir string) string { return filepath.Join(userDir, userAgentsMdFileName) }
func userAgentsAdminMd(userDir string) string {
	return filepath.Join(userDir, userAgentsAdminMdFileName)
}

func userAgentsReadmeMd(userDir string) string {
	return filepath.Join(userDir, userAgentsReadmeMdFileName)
}

func claudeScratch(userDir string) string {
	return filepath.Join(userDir, "tmp", "claude", AgentsMdFileName)
}

func claudeBase(userDir string) string {
	return filepath.Join(userDir, "tmp", "claude."+AgentsMdFileName+".orig")
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

// lstatGone is gone without following symlinks
func lstatGone(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	return os.IsNotExist(err)
}

// linkFile creates ccbox's symlink at path, pointing at target
func linkFile(t *testing.T, path, target string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), ioutil.Dir))
	require.NoError(t, os.Symlink(target, path))
}

// assertSymlinked asserts the realCliFile is ccbox's own symlink into the scratch mount
func assertSymlinked(t *testing.T, cliFile string) {
	t.Helper()
	info, err := os.Lstat(cliFile)
	require.NoError(t, err)
	require.NotEqual(t, 0, info.Mode()&os.ModeSymlink, "a symlink, not a regular file")
	target, err := os.Readlink(cliFile)
	require.NoError(t, err)
	assert.Equal(t, claudeTarget(), target)
}

// assertRegular asserts path is a regular file, not a symlink
func assertRegular(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	require.NoError(t, err)
	assert.True(t, info.Mode().IsRegular(), "a regular file, not a symlink")
}

func TestSafeSeedAgentsMd(t *testing.T) {
	for _, tc := range []struct {
		caseName string
		setup    func(t *testing.T, userDir string) // written before the seed
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
				assert.True(t, gone(t, userAgentsAdminMd(userDir)), "the admin variant is not seeded")
			} else {
				assert.Equal(t, *tc.admin, readFile(t, userAgentsAdminMd(userDir)))
			}
			if tc.readme == nil {
				assert.True(t, gone(t, userAgentsReadmeMd(userDir)), "the explainer is not seeded")
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
		err      bool   // whether the share errors; no scratch is written
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
			caseName: "CliFileIsCcboxSymlink",
			setup: func(t *testing.T, userDir string) {
				linkFile(t, claudeCliFile(userDir), claudeTarget()) // ccbox's symlink, left by a past run
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
				scratchFile, cleanup, err := claudeShare().Begin()
				require.Error(t, err)
				assert.Empty(t, scratchFile)
				assert.True(t, gone(t, claudeScratch(userDir)), "no scratch written")
				assert.True(t, gone(t, claudeBase(userDir)), "no baseline written")
				require.NoError(t, cleanup())
				return
			}

			claudeShareBegin(t, func(scratchFile string) {
				if !tc.binds {
					assert.Empty(t, scratchFile)
					assert.True(t, gone(t, claudeScratch(userDir)), "no scratch written")
					assert.True(t, gone(t, claudeBase(userDir)), "no baseline written")
					return
				}
				assert.Equal(t, claudeScratch(userDir), scratchFile)
				assert.Equal(t, tc.body, readFile(t, claudeScratch(userDir)), "the bound scratch's body")
				assert.Equal(t, tc.body, readFile(t, claudeBase(userDir)), "the written baseline's body")
				assertSymlinked(t, claudeCliFile(userDir))
			})
		})
	}
}

func TestAgentsMdShare_Begin_EmptyCliFileIsOwned(t *testing.T) {
	userDir := resetUserDir(t)
	writeFile(t, claudeCliFile(userDir), "")

	scratchFile, cleanup, err := claudeShare().Begin()
	require.NoError(t, err)

	assert.Empty(t, scratchFile, "an empty realCliFile is the user's own doc")
	assert.True(t, gone(t, claudeScratch(userDir)), "no scratch written")

	require.NoError(t, cleanup())
	assert.Empty(t, readFile(t, claudeCliFile(userDir)), "the empty doc is untouched")
}

func TestAgentsMdShare_Begin_ForeignSymlinkIsOwned(t *testing.T) {
	userDir := resetUserDir(t)
	linkFile(t, claudeCliFile(userDir), "/elsewhere/AGENTS.md") // e.g. the user's dotfiles repo

	scratchFile, cleanup, err := claudeShare().Begin()
	require.NoError(t, err)

	assert.Empty(t, scratchFile, "a foreign symlink is the user's own doc")
	assert.True(t, gone(t, claudeScratch(userDir)), "no scratch written")

	require.NoError(t, cleanup())
	target, err := os.Readlink(claudeCliFile(userDir))
	require.NoError(t, err)
	assert.Equal(t, "/elsewhere/AGENTS.md", target, "the foreign symlink is untouched")
}

func TestAgentsMdShare_Begin_Piggyback(t *testing.T) {
	userDir := resetUserDir(t)
	writeFile(t, userAgentsMd(userDir), "shared")

	// a live run binds the share
	_, clean, err := claudeShare().Begin()
	require.NoError(t, err)

	// a second run piggybacks: the same scratch binds, the written share untouched
	scratchFile, piggybackClean, err := claudeShare().Begin()
	require.NoError(t, err)
	assert.Equal(t, claudeScratch(userDir), scratchFile)
	assert.Equal(t, "shared", readFile(t, claudeScratch(userDir)))
	assert.Equal(t, "shared", readFile(t, claudeBase(userDir)))
	assertSymlinked(t, claudeCliFile(userDir))

	// the piggyback's cleanup leaves the share for the live run
	require.NoError(t, piggybackClean())
	assert.FileExists(t, claudeScratch(userDir))
	assertSymlinked(t, claudeCliFile(userDir))

	// the last run out promotes and removes the share
	writeFile(t, claudeScratch(userDir), "memory")
	require.NoError(t, clean())
	assert.Equal(t, "memory", readFile(t, claudeCliFile(userDir)))
	assertRegular(t, claudeCliFile(userDir))
	assert.True(t, gone(t, claudeScratch(userDir)))
	assert.True(t, gone(t, claudeBase(userDir)))
}

// TestAgentsMdShare_Begin_LatecomerBindsOwnDoc: a latecomer joining a live scratch session
// whose host has written its own realCliFile mid-session binds nothing, and the live run's share
// is left alone
func TestAgentsMdShare_Begin_LatecomerBindsOwnDoc(t *testing.T) {
	userDir := resetUserDir(t)
	writeFile(t, userAgentsMd(userDir), "shared")

	// a live run binds the share
	_, clean, err := claudeShare().Begin()
	require.NoError(t, err)

	// the host replaces the share's symlink with its own doc mid-session
	require.NoError(t, os.Remove(claudeCliFile(userDir)))
	writeFile(t, claudeCliFile(userDir), "cli")

	// the latecomer binds nothing
	scratchFile, piggybackClean, err := claudeShare().Begin()
	require.NoError(t, err)
	assert.Empty(t, scratchFile)
	assert.Equal(t, "cli", readFile(t, claudeCliFile(userDir)))

	// the live run's share is untouched until its last out settles
	require.NoError(t, piggybackClean())
	assert.FileExists(t, claudeScratch(userDir))
	assert.FileExists(t, claudeBase(userDir))

	// the last out settles nothing: the scratch was never edited, and the host's doc stands
	require.NoError(t, clean())
	assert.Equal(t, "cli", readFile(t, claudeCliFile(userDir)))
	assertRegular(t, claudeCliFile(userDir))
	assert.True(t, gone(t, claudeScratch(userDir)))
	assert.True(t, gone(t, claudeBase(userDir)))
}

// TestAgentsMdShare_Begin_LatecomerJoinsOwnDoc: the first run binds the CLI's own doc,
// no scratch written; the latecomer joins the doc all the same
func TestAgentsMdShare_Begin_LatecomerJoinsOwnDoc(t *testing.T) {
	userDir := resetUserDir(t)
	writeFile(t, claudeCliFile(userDir), "cli")

	_, clean, err := claudeShare().Begin()
	require.NoError(t, err)
	assert.True(t, gone(t, claudeScratch(userDir)), "an owned doc binds directly, no scratch written")

	scratchFile, piggybackClean, err := claudeShare().Begin()
	require.NoError(t, err)
	assert.Empty(t, scratchFile)
	assert.Equal(t, "cli", readFile(t, claudeCliFile(userDir)))

	require.NoError(t, piggybackClean())
	require.NoError(t, clean())
	assert.Equal(t, "cli", readFile(t, claudeCliFile(userDir)))
	assert.True(t, gone(t, claudeScratch(userDir)))
}

func TestAgentsMdShare_VerifyShared(t *testing.T) {
	userDir := resetUserDir(t)
	s := claudeShare()

	require.Error(t, s.verifyShared(), "nothing present: neither the scratch nor the doc")

	writeFile(t, claudeCliFile(userDir), "cli")
	require.NoError(t, s.verifyShared(), "the owned doc bound directly is the share")
}

func TestAgentsMdShare_Begin_SettlesLeftover_PromotesChanged(t *testing.T) {
	userDir := resetUserDir(t)
	writeFile(t, claudeScratch(userDir), "edited") // leftover scratch: edited by the crashed run

	claudeShareBegin(t, func(scratchFile string) {
		assert.Empty(t, scratchFile) // leftover promoted, do nothing
	})

	// check promotion
	assert.Equal(t, "edited", readFile(t, claudeCliFile(userDir)))
	assertRegular(t, claudeCliFile(userDir))
	assert.True(t, gone(t, claudeScratch(userDir)))
}

func TestAgentsMdShare_Begin_SettlesLeftover_ResharesUnchanged(t *testing.T) {
	userDir := resetUserDir(t)

	// leftover scratch: unchanged from its source doc
	writeFile(t, userAgentsMd(userDir), "shared")
	writeFile(t, claudeScratch(userDir), "shared")

	// during session: re-bound as a fresh scratch
	claudeShareBegin(t, func(scratchFile string) {
		assert.Equal(t, claudeScratch(userDir), scratchFile)
		assert.Equal(t, "shared", readFile(t, claudeScratch(userDir)))
		assertSymlinked(t, claudeCliFile(userDir))
	})

	assert.True(t, lstatGone(t, claudeCliFile(userDir))) // symlink dropped
	assert.True(t, gone(t, claudeScratch(userDir)))      // scratch gone
	assert.True(t, gone(t, claudeBase(userDir)))         // baseline gone
}

// TestAgentsMdShare_Begin_SettlesLeftover_AfterPromote: a crash between the promote and the
// cleanup's removals leaves the promoted doc, its scratch, and the baseline; the next run
// finishes the settle and binds nothing
func TestAgentsMdShare_Begin_SettlesLeftover_AfterPromote(t *testing.T) {
	userDir := resetUserDir(t)
	writeFile(t, userAgentsMd(userDir), "shared")
	writeFile(t, claudeScratch(userDir), "edited")
	writeFile(t, claudeBase(userDir), "shared")
	writeFile(t, claudeCliFile(userDir), "edited") // the promoted edit

	claudeShareBegin(t, func(scratchFile string) {
		assert.Empty(t, scratchFile)
	})

	assert.Equal(t, "edited", readFile(t, claudeCliFile(userDir)))
	assertRegular(t, claudeCliFile(userDir))
	assert.True(t, gone(t, claudeScratch(userDir)))
	assert.True(t, gone(t, claudeBase(userDir)))
}

// crashRun binds the claude share in a child process that exits without cleanup: the
// kernel releases the crashed run's registry entry, as a real crash does
func crashRun(t *testing.T, userDir string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHelperCrashRun$") // #nosec G204,G702 -- re-execs the test binary itself
	cmd.Env = append(os.Environ(), "HOME="+filepath.Dir(userDir), "CCBOX_CRASH_RUN=1")    // userDir is ~/.ccbox
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "child output:\n%s", out)
}

// TestHelperCrashRun is crashRun's child process
func TestHelperCrashRun(t *testing.T) {
	if os.Getenv("CCBOX_CRASH_RUN") != "1" {
		t.Skip("helper: not under test")
	}
	_, _, err := claudeShare().Begin()
	require.NoError(t, err)
}

// TestAgentsMdShare_Cleanup settles the bound scratch into the realCliFile: a diff is promoted
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

			claudeShareBegin(t, func(_ string) {
				if tc.scratchChange != "" {
					writeFile(t, claudeScratch(userDir), tc.scratchChange)
				}
			})

			if tc.expectedClaudeFile == nil {
				assert.True(t, lstatGone(t, claudeCliFile(userDir)), "nothing promoted; ccbox's symlink is dropped")
			} else {
				assert.Equal(t, *tc.expectedClaudeFile, readFile(t, claudeCliFile(userDir)))
				assertRegular(t, claudeCliFile(userDir))
			}
			assert.True(t, gone(t, claudeScratch(userDir)), "scratch removed")
			assert.True(t, gone(t, claudeBase(userDir)), "baseline removed")
			assert.Equal(t, protectedBody, readFile(t, protectedPath), "the source doc is untouched")
		})
	}
}

func TestAgentsMdShare_Cleanup_LoneSymlink(t *testing.T) {
	userDir := resetUserDir(t)
	linkFile(t, claudeCliFile(userDir), claudeTarget()) // leftover symlink: its scratch is already gone

	require.NoError(t, claudeShare().clean())

	assert.True(t, lstatGone(t, claudeCliFile(userDir)), "the lone symlink is dropped")
}

func TestAgentsMdShare_Cleanup_DiffErrorDropsSymlink(t *testing.T) {
	userDir := resetUserDir(t)
	linkFile(t, claudeCliFile(userDir), claudeTarget())
	writeFile(t, claudeScratch(userDir), "leftover")
	require.NoError(t, os.Remove(userAgentsAdminMd(userDir))) // no source doc: the diff errors

	require.Error(t, claudeShare().clean())

	assert.True(t, lstatGone(t, claudeCliFile(userDir)), "the symlink is dropped despite the failed settle")
	assert.False(t, gone(t, claudeScratch(userDir)), "the scratch is kept for the next run")
}

func TestAgentsMdShare_Cleanup_KeepsScratchOnPromoteError(t *testing.T) {
	userDir := resetUserDir(t)
	_, cleanup, err := claudeShare().Begin()
	require.NoError(t, err)
	writeFile(t, claudeScratch(userDir), "memory")
	require.NoError(t, os.Remove(claudeCliFile(userDir)))               // clear ccbox's symlink
	require.NoError(t, os.MkdirAll(claudeCliFile(userDir), ioutil.Dir)) // a directory is unreadable: promotion fails

	require.Error(t, cleanup(), "realCliFile is a directory")
	assert.Equal(t, "memory", readFile(t, claudeScratch(userDir)), "scratch kept for the next run")
	assert.Equal(t, adminMd, readFile(t, claudeBase(userDir)), "baseline kept with the scratch")
}

// TestAgentsMdShare_Cleanup_HostChangedDuringRun: the host's doc edits settle nothing; the
// container's scratch edits still promote
func TestAgentsMdShare_Cleanup_HostChangedDuringRun(t *testing.T) {
	for _, tc := range []struct {
		caseName           string
		setup              func(t *testing.T, userDir string)
		scratchEdit        string                             // written to the bound scratch during the run
		hostEdit           func(t *testing.T, userDir string) // a host-side change during the run
		check              func(t *testing.T, userDir string) // the shared docs' state after
		expectedClaudeFile *string                            // the expected cliFile body after the cleanup
	}{
		{
			caseName: "UserDocEdited",
			hostEdit: func(t *testing.T, userDir string) {
				writeFile(t, userAgentsMd(userDir), "host edit")
			},
			check: func(t *testing.T, userDir string) {
				assert.Equal(t, "host edit", readFile(t, userAgentsMd(userDir)))
			},
		},
		{
			caseName: "AdminDocEdited",
			hostEdit: func(t *testing.T, userDir string) {
				writeFile(t, userAgentsAdminMd(userDir), "host admin")
			},
			check: func(t *testing.T, userDir string) {
				assert.Equal(t, "host admin", readFile(t, userAgentsAdminMd(userDir)))
			},
		},
		{
			caseName: "UserDocDeleted",
			setup: func(t *testing.T, userDir string) {
				writeFile(t, userAgentsMd(userDir), "shared")
			},
			hostEdit: func(t *testing.T, userDir string) {
				require.NoError(t, os.Remove(userAgentsMd(userDir)))
			},
			check: func(t *testing.T, userDir string) {
				assert.True(t, gone(t, userAgentsMd(userDir)))
			},
		},
		{
			caseName: "AdminDocDeleted",
			hostEdit: func(t *testing.T, userDir string) {
				require.NoError(t, os.Remove(userAgentsAdminMd(userDir)))
			},
			check: func(t *testing.T, userDir string) {
				assert.True(t, gone(t, userAgentsAdminMd(userDir)))
			},
		},
		{
			caseName: "UserDocEditedAndContainerEdited",
			setup: func(t *testing.T, userDir string) {
				writeFile(t, userAgentsMd(userDir), "shared")
			},
			scratchEdit: "memory",
			hostEdit: func(t *testing.T, userDir string) {
				writeFile(t, userAgentsMd(userDir), "host edit")
			},
			check: func(t *testing.T, userDir string) {
				assert.Equal(t, "host edit", readFile(t, userAgentsMd(userDir)))
			},
			expectedClaudeFile: new("memory"),
		},
	} {
		t.Run(tc.caseName, func(t *testing.T) {
			userDir := resetUserDir(t)
			if tc.setup != nil {
				tc.setup(t, userDir)
			}

			// during the run: the container edits its scratch, then the host edits its doc
			claudeShareBegin(t, func(_ string) {
				if tc.scratchEdit != "" {
					writeFile(t, claudeScratch(userDir), tc.scratchEdit)
				}
				tc.hostEdit(t, userDir)
			})

			// the host's edit stands
			tc.check(t, userDir)

			// a container-edited scratch promotes; an unchanged one is dropped
			if tc.expectedClaudeFile == nil {
				assert.True(t, lstatGone(t, claudeCliFile(userDir)))
			} else {
				assert.Equal(t, *tc.expectedClaudeFile, readFile(t, claudeCliFile(userDir)))
				assertRegular(t, claudeCliFile(userDir))
			}
			assert.True(t, gone(t, claudeScratch(userDir)))
			assert.True(t, gone(t, claudeBase(userDir)))
		})
	}
}

// TestAgentsMdShare_Cleanup_HostChangedDuringCrashedRun: the leftover settle diffs against the
// crashed run's baseline, so host edits made while down are not promoted
func TestAgentsMdShare_Cleanup_HostChangedDuringCrashedRun(t *testing.T) {
	userDir := resetUserDir(t)

	// a crashed run leaves its scratch, baseline, and symlink
	crashRun(t, userDir)

	// the host edits while it's down
	writeFile(t, userAgentsMd(userDir), "host edit")

	// the next run settles the leftovers: nothing promoted
	require.NoError(t, claudeShare().clean())
	assert.True(t, lstatGone(t, claudeCliFile(userDir)))
	assert.True(t, gone(t, claudeScratch(userDir)))
	assert.True(t, gone(t, claudeBase(userDir)))

	// re-binds the host's fresh doc
	claudeShareBegin(t, func(scratchFile string) {
		assert.Equal(t, claudeScratch(userDir), scratchFile)
		assert.Equal(t, "host edit", readFile(t, claudeScratch(userDir)))
		assert.Equal(t, "host edit", readFile(t, claudeBase(userDir)))
	})
}

func TestAgentsMdShare_Cleanup_LoneBaseline(t *testing.T) {
	userDir := resetUserDir(t)

	// a crash left its baseline; the scratch is gone
	writeFile(t, claudeBase(userDir), adminMd)

	// the lone baseline is removed
	require.NoError(t, claudeShare().clean())
	assert.True(t, gone(t, claudeBase(userDir)))
}
