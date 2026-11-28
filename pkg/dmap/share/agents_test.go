package share

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

// The tests cover AgentsMd's own wiring — the source doc it binds and the paths it
// lands on. The sharing itself is tested in pkg/util/scratch.

// resetUserDir points userdir at a fresh temp tree with the shared AGENTS docs seeded
func resetUserDir(t *testing.T) string {
	t.Helper()
	testutil.Home(t)
	require.NoError(t, SafeSeedAgentsMd())
	return userdir.Dir()
}

// piMount is the scratch file's container path, as AgentsMd wires it
const piMount = "/home/ccbox/.ccbox/tmp/agents/pi/AGENTS.md"

func piTarget() string { return piMount }

// piShareBegin begins a bound share for pi, runs body with the bound scratch file,
// then cleans up
func piShareBegin(t *testing.T, isProxied bool, body func(scratchFile string)) {
	t.Helper()
	bind, cleanup, err := AgentsMd("pi", isProxied).Begin()
	require.NoError(t, err)
	if body != nil {
		body(bind.HostPath)
	}
	require.NoError(t, cleanup())
}

func piCliFile(userDir string) string { return filepath.Join(userDir, "pi", agentsMdFileName) }

func piTmplMd(userDir string) string {
	return filepath.Join(userDir, "pi", agentsTmplMdFileName)
}

func userAgentsTmplMd(userDir string) string {
	return filepath.Join(userDir, agentsTmplMdFileName)
}

func piScratch(userDir string) string {
	return filepath.Join(userDir, "tmp", "agents", "pi", agentsMdFileName)
}

func piBase(userDir string) string { return piScratch(userDir) + ".orig" }

func renderTmpl(t *testing.T, ctx agentsTmplCtx, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), agentsTmplMdFileName)
	writeFile(t, path, body)
	rendered, err := renderAgentsTmpl(path, ctx)
	require.NoError(t, err)
	return string(rendered)
}

// renderedDefault renders the seeded default template, as the shared doc does
func renderedDefault(t *testing.T, isProxied bool) string {
	t.Helper()
	return renderTmpl(t, agentsTmplCtx{isProxied: isProxied, isShared: true}, string(defaultTmplMd))
}

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
	assert.Equal(t, piTarget(), target)
}

func TestSafeSeedAgentsMd(t *testing.T) {
	for _, tc := range []struct {
		caseName string
		setup    func(t *testing.T, userDir string) // written before the seed
		tmpl     *string                            // the template's body after the seed; nil when gone
	}{
		{
			caseName: "LaysDefault",
			tmpl:     new(string(defaultTmplMd)),
		},
		{
			caseName: "SkipsExistingTmpl",
			setup: func(t *testing.T, userDir string) {
				writeFile(t, userAgentsTmplMd(userDir), "kept")
			},
			tmpl: new("kept"),
		},
	} {
		t.Run(tc.caseName, func(t *testing.T) {
			testutil.Home(t)
			userDir := userdir.Dir()
			if tc.setup != nil {
				tc.setup(t, userDir)
			}

			require.NoError(t, SafeSeedAgentsMd())

			if tc.tmpl == nil {
				assert.True(t, gone(t, userAgentsTmplMd(userDir)), "the template is not seeded")
			} else {
				assert.Equal(t, *tc.tmpl, readFile(t, userAgentsTmplMd(userDir)))
			}
		})
	}
}

// TestAgentsMd_PromotesRendered binds the most specific template's render; edits
// to it promote to the CLI doc
func TestAgentsMd_PromotesRendered(t *testing.T) {
	for _, tc := range []struct {
		caseName string
		doc      func(string) string // the template the source resolves to
		body     string              // the template's body
	}{
		{caseName: "SharedTmpl", doc: userAgentsTmplMd, body: "<!--{{ adminSection }}-->\nadmin rules"},
		{caseName: "CliTmpl", doc: piTmplMd, body: "<!--{{ sharedAgentsMd }}-->\ncli rules"},
	} {
		t.Run(tc.caseName, func(t *testing.T) {
			userDir := resetUserDir(t)
			writeFile(t, tc.doc(userDir), tc.body)
			rendered := renderTmpl(t, agentsTmplCtx{isProxied: true}, tc.body)

			piShareBegin(t, true, func(string) {
				writeFile(t, piScratch(userDir), rendered+", edited")
			})

			assert.Equal(t, rendered+", edited", readFile(t, piCliFile(userDir)))
		})
	}
}

// TestAdminMd checks the default template's rendered doc against the testdata fixtures.
// Regenerate with `UPDATE_FIXTURES=1 go test ./pkg/dmap/share/ -run TestAdminMd`
func TestAdminMd(t *testing.T) {
	for _, tt := range []struct {
		name      string // fixture name: testdata/TestAdminMd_<name>.md
		isProxied bool
	}{
		{"proxy", true},     // the network section renders
		{"no-proxy", false}, // the network section is dropped
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := renderedDefault(t, tt.isProxied)

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
		setup    func(t *testing.T, userDir string) // written before the share
		binds    bool                               // whether the share binds a scratch
		err      bool                               // whether the share errors; no scratch is written
		body     func(t *testing.T) string          // the bound scratch's body; unset when !binds
	}{
		{
			caseName: "PlainUserTmpl",
			setup: func(t *testing.T, userDir string) {
				writeFile(t, userAgentsTmplMd(userDir), "shared")
			},
			binds: true,
			body:  func(*testing.T) string { return "shared" },
		},
		{
			caseName: "SeededDefault",
			binds:    true,
			body:     func(t *testing.T) string { return renderedDefault(t, true) },
		},
		{
			caseName: "CliTmplOverridesShared",
			setup: func(t *testing.T, userDir string) {
				writeFile(t, userAgentsTmplMd(userDir), "shared")
				writeFile(t, piTmplMd(userDir), "<!--{{ adminSection }}-->\ncli rules")
			},
			binds: true,
			body: func(t *testing.T) string {
				return renderTmpl(t, agentsTmplCtx{isProxied: true}, "<!--{{ adminSection }}-->\ncli rules")
			},
		},
		{
			caseName: "SkipsWhenCliFileExists",
			setup: func(t *testing.T, userDir string) {
				writeFile(t, piCliFile(userDir), "cli")
			},
		},
		{
			caseName: "ErrorsWhenNoSourceDoc",
			setup: func(t *testing.T, userDir string) {
				require.NoError(t, os.Remove(userAgentsTmplMd(userDir)))
			},
			err: true,
		},
		{
			caseName: "ErrorsWhenTmplInvalid",
			setup: func(t *testing.T, userDir string) {
				writeFile(t, userAgentsTmplMd(userDir), "<!--{{ nope }}-->")
			},
			err: true,
		},
		{
			caseName: "ErrorsWhenSharedAgentsMdRecurses",
			setup: func(t *testing.T, userDir string) {
				writeFile(t, userAgentsTmplMd(userDir), "<!--{{ sharedAgentsMd }}-->")
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
				bind, cleanup, err := AgentsMd("pi", true).Begin()
				require.Error(t, err)
				assert.Empty(t, bind.HostPath)
				assert.True(t, gone(t, piScratch(userDir)), "no scratch written")
				assert.True(t, gone(t, piBase(userDir)), "no baseline written")
				require.NoError(t, cleanup())
				return
			}

			piShareBegin(t, true, func(scratchFile string) {
				if !tc.binds {
					assert.Empty(t, scratchFile)
					assert.True(t, gone(t, piScratch(userDir)), "no scratch written")
					assert.True(t, gone(t, piBase(userDir)), "no baseline written")
					return
				}
				assert.Equal(t, piScratch(userDir), scratchFile)
				assert.Equal(t, tc.body(t), readFile(t, piScratch(userDir)), "the bound scratch's body")
				assert.Equal(t, tc.body(t), readFile(t, piBase(userDir)), "the written baseline's body")
				assertSymlinked(t, piCliFile(userDir))
			})
			assert.True(t, gone(t, piScratch(userDir)), "the cleanup drops the share")
			assert.True(t, gone(t, piBase(userDir)), "the cleanup drops the baseline")
		})
	}
}
