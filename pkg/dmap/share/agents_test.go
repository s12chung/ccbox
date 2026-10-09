package share

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/scratch"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

// The tests pin AgentsMd's own wiring — the source doc it binds and the paths it
// lands on. The driver's behavior is pinned in pkg/util/scratch.

// resetUserDir points userdir at a fresh temp tree with the shared AGENTS docs seeded
func resetUserDir(t *testing.T) string {
	t.Helper()
	testutil.Home(t)
	require.NoError(t, SafeSeedAgentsMd())
	return userdir.Dir()
}

// claudeMount is the scratch file's container path, as AgentsMd wires it
const claudeMount = "/home/ccbox/.ccbox/tmp/agents/claude/AGENTS.md"

func claudeShare() scratch.Binder {
	return AgentsMd("claude", false)
}

// claudeTarget is the CLI doc symlink's target
func claudeTarget() string { return claudeMount }

// claudeShareBegin begins a bound share for claude, runs body with the bound scratch file,
// then cleans up
func claudeShareBegin(t *testing.T, body func(scratchFile string)) {
	t.Helper()
	bind, cleanup, err := claudeShare().Begin()
	require.NoError(t, err)
	if body != nil {
		body(bind.HostPath)
	}
	require.NoError(t, cleanup())
}

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
	return filepath.Join(userDir, "tmp", "agents", "claude", AgentsMdFileName)
}

func claudeBase(userDir string) string { return claudeScratch(userDir) + ".orig" }

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), osutil.Dir))
	require.NoError(t, os.WriteFile(path, []byte(body), osutil.File))
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

// assertSymlinked asserts the CLI doc is ccbox's own symlink into the scratch mount
func assertSymlinked(t *testing.T, cliFile string) {
	t.Helper()
	info, err := os.Lstat(cliFile)
	require.NoError(t, err)
	require.NotEqual(t, 0, info.Mode()&os.ModeSymlink, "a symlink, not a regular file")
	target, err := os.Readlink(cliFile)
	require.NoError(t, err)
	assert.Equal(t, claudeTarget(), target)
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
			readme:   new(string(agentsReadmeMd)),
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
			testutil.Home(t)
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

// TestAgentsMd_PromotesPrefixed binds the share's source doc, most specific first
func TestAgentsMd_PromotesPrefixed(t *testing.T) {
	for _, tc := range []struct {
		caseName string
		doc      func(string) string // the ccbox-admin variant the source resolves to
		body     string              // the variant's body
	}{
		{caseName: "SharedAdmin", doc: userAgentsAdminMd, body: "admin rules"},
		{caseName: "CliAdmin", doc: claudeAdminMd, body: "cli rules"},
	} {
		t.Run(tc.caseName, func(t *testing.T) {
			userDir := resetUserDir(t)
			writeFile(t, tc.doc(userDir), tc.body)

			claudeShareBegin(t, func(string) {
				writeFile(t, claudeScratch(userDir), adminMd(false)+tc.body+", edited")
			})

			assert.Equal(t, adminMd(false)+tc.body+", edited", readFile(t, claudeCliFile(userDir)))
		})
	}
}

// TestAdminMd pins the rendered admin doc to the committed testdata fixtures — each
// render must stay byte-identical to its fixture. Regenerate with:
// `UPDATE_FIXTURES=1 go test ./pkg/dmap/share/ -run TestAdminMd`
func TestAdminMd(t *testing.T) {
	for _, tt := range []struct {
		name      string // fixture name: testdata/TestAdminMd_<name>.md
		isNoProxy bool
	}{
		{"proxy", false},   // the network section rides
		{"no-proxy", true}, // the network section is dropped
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := adminMd(tt.isNoProxy)

			path := filepath.Join("testdata", "TestAdminMd_"+tt.name+".md")
			if os.Getenv("UPDATE_FIXTURES") != "" {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), osutil.Dir))
				require.NoError(t, os.WriteFile(path, []byte(got), osutil.File))
			}

			// #nosec G304 -- the package's own fixture path
			want, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, string(want), got)
		})
	}
}

func TestAgentsMd_Begin(t *testing.T) {
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
			body:  adminMd(false) + "admin rules",
		},
		{
			caseName: "SeededDefault",
			binds:    true,
			body:     adminMd(false),
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
			body:  adminMd(false) + "cli rules",
		},
		{
			caseName: "SkipsWhenCliFileExists",
			setup: func(t *testing.T, userDir string) {
				writeFile(t, claudeCliFile(userDir), "cli")
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
				bind, cleanup, err := claudeShare().Begin()
				require.Error(t, err)
				assert.Empty(t, bind.HostPath)
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
			assert.True(t, gone(t, claudeScratch(userDir)), "the cleanup drops the share")
			assert.True(t, gone(t, claudeBase(userDir)), "the cleanup drops the baseline")
		})
	}
}
