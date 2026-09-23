package harness

import (
	"io/fs"
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/must"
)

func TestMain(m *testing.M) {
	// ignore any user clis on this machine: tests pin the embedded set
	embedClis := must.Get(wrapTreeLoad(embedTree().load()))
	all = make(map[string]CLI, len(embedClis))
	for _, c := range embedClis {
		all[c.Name] = c
	}
	os.Exit(m.Run())
}

func TestNames(t *testing.T) {
	assert.Equal(t, []string{"claude", "codex", "grok", "opencode", "pi"}, Names())
}

func TestSeedCLIFS(t *testing.T) {
	for _, c := range All() {
		fsys := SeedCLIFS(c.Name)
		assert.NotContainsf(t, seedPaths(t, fsys), "AGENTS.md",
			"shared AGENTS doc is bind-mounted at runtime, never seeded: %s", c.Name)
	}

	claudeFS := SeedCLIFS("claude") // per-CLI tree rooted at its config dir
	want := []string{"hooks/secret-tripwire.sh", "settings.json", "statusline.sh"}
	assert.Equal(t, want, seedPaths(t, claudeFS))

	assert.PanicsWithValue(t, `harness: unknown cli "emacs"`, func() { SeedCLIFS("emacs") })
}

func TestSeedCLIFS_UserTree(t *testing.T) {
	dir := resetAll(t)
	writeUserCli(t, dir, "mycli", userCliYAML, map[string]string{"config/settings.toml": "[x]\n"})
	writeUserCli(t, dir, "bare", userCliYAML, nil)
	all = mustLoadAll()

	fsys := SeedCLIFS("mycli")
	assert.Equal(t, []string{"settings.toml"}, seedPaths(t, fsys), "user config tree only")

	// no config tree laid down: SeedCLIFS mkdirs an empty one
	bareFS := SeedCLIFS("bare")
	assert.Empty(t, seedPaths(t, bareFS))
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

func TestFor(t *testing.T) {
	for _, want := range All() {
		got, ok := For(want.Name)
		assert.True(t, ok, want.Name)
		assert.Equal(t, want, got)
	}

	got, ok := For("emacs")
	assert.False(t, ok, "unknown name")
	assert.Zero(t, got)
}

func TestMust_For(t *testing.T) {
	for _, want := range All() {
		assert.Equal(t, want, MustFor(want.Name))
	}
	assert.PanicsWithValue(t, `harness: unknown cli "emacs"`, func() { MustFor("emacs") })
}

func TestCLI_PkgInfoJSON(t *testing.T) {
	// the CLI_PKGINFO env JSON each CLI's install source renders
	body, err := MustFor("claude").PkgInfoJSON()
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"claude","npm":{"package":"@anthropic-ai/claude-code"},"version_url":null}`, body)

	body, err = MustFor("grok").PkgInfoJSON()
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"grok","npm":null,"version_url":{"url":"https://x.ai/cli/stable",`+
		`"linux_x64_url":"https://x.ai/cli/grok-$version-linux-x86_64",`+
		`"linux_arm64_url":"https://x.ai/cli/grok-$version-linux-aarch64"}}`, body)
}

func TestCLI_SessionCmd(t *testing.T) {
	tests := []struct {
		cli    CLI
		shell  bool
		cont   bool
		resume bool
		args   []string
		want   []string
	}{
		{cli: MustFor("claude"), shell: true, want: nil},
		{cli: MustFor("claude"), want: []string{"claude"}},
		{cli: MustFor("claude"), cont: true, want: []string{"claude", "-c"}},
		{cli: MustFor("claude"), resume: true, want: []string{"claude", "--resume"}},
		{cli: MustFor("claude"), resume: true, args: []string{"auth-refactor"}, want: []string{"claude", "--resume", "auth-refactor"}},

		{cli: MustFor("codex"), want: []string{"codex", "--sandbox", "danger-full-access"}},
		{cli: MustFor("codex"), cont: true, want: []string{"codex", "--sandbox", "danger-full-access", "resume", "--last"}},
		{cli: MustFor("codex"), resume: true, want: []string{"codex", "--sandbox", "danger-full-access", "resume"}},
		{cli: MustFor("codex"), resume: true, args: []string{"abc123"}, want: []string{"codex", "--sandbox", "danger-full-access", "resume", "abc123"}},

		{cli: MustFor("opencode"), want: []string{"opencode"}},
		{cli: MustFor("opencode"), cont: true, want: []string{"opencode", "-c"}},
		// No picker flag: bare --session errors in opencode, so -r needs an id
		{cli: MustFor("opencode"), resume: true, want: []string{"opencode", "--session"}},
		{cli: MustFor("opencode"), resume: true, args: []string{"ses_42"}, want: []string{"opencode", "--session", "ses_42"}},

		{cli: MustFor("grok"), want: []string{"grok"}},
		{cli: MustFor("grok"), cont: true, want: []string{"grok", "-c"}},
		// Bare --resume resumes the most recent session
		{cli: MustFor("grok"), resume: true, want: []string{"grok", "--resume"}},
		{cli: MustFor("grok"), resume: true, args: []string{"abc-uuid"}, want: []string{"grok", "--resume", "abc-uuid"}},

		{cli: MustFor("pi"), want: []string{"pi"}},
		{cli: MustFor("pi"), cont: true, want: []string{"pi", "-c"}},
		// No picker flag: bare --session errors, so -r needs an id
		{cli: MustFor("pi"), resume: true, want: []string{"pi", "--session"}},
		{cli: MustFor("pi"), resume: true, args: []string{"abc123"}, want: []string{"pi", "--session", "abc123"}},
	}
	for _, tt := range tests {
		got := tt.cli.SessionCmd(tt.shell, tt.cont, tt.resume, tt.args)
		assert.Equal(t, tt.want, got, "%s shell=%v cont=%v resume=%v", tt.cli.Name, tt.shell, tt.cont, tt.resume)
	}
}

func TestEnv(t *testing.T) {
	// Config-dir overrides point at the CLI's native config mount; toggles disable
	// update checks / nonessential traffic. pkg/docker merges these into every run.
	assert.Equal(t, "CLAUDE_CONFIG_DIR", *MustFor("claude").ConfigDirEnvKey)
	assert.Equal(t, map[string]string{
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
		"DISABLE_AUTOUPDATER":                      "1",
	}, MustFor("claude").Env)

	assert.Equal(t, "CODEX_HOME", *MustFor("codex").ConfigDirEnvKey)
	assert.Empty(t, MustFor("codex").Env)

	assert.Nil(t, MustFor("opencode").ConfigDirEnvKey)
	assert.Equal(t, map[string]string{"OPENCODE_DISABLE_AUTOUPDATE": "1"}, MustFor("opencode").Env)

	assert.Nil(t, MustFor("grok").ConfigDirEnvKey)
	assert.Equal(t, map[string]string{"GROK_DISABLE_AUTOUPDATER": "1"}, MustFor("grok").Env)

	assert.Nil(t, MustFor("pi").ConfigDirEnvKey)
	assert.Equal(t, map[string]string{"PI_SKIP_VERSION_CHECK": "1"}, MustFor("pi").Env)
}

func TestCLI_CLIDataBinds(t *testing.T) {
	authJSON := "{}"
	assert.Equal(t,
		map[string]*string{".local/share/opencode/auth.json": &authJSON},
		MustFor("opencode").DataBinds)

	for _, name := range []string{"claude", "codex", "grok", "pi"} {
		assert.Emptyf(t, MustFor(name).DataBinds, "%s has no data binds", name)
	}
}
