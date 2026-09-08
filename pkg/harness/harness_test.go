package harness

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/fsutil"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

func TestMain(m *testing.M) {
	all = mustLoadEmbedCLIs() // ignore any user clis on this machine: tests pin the embedded set
	os.Exit(m.Run())
}

// resetAll redirects the user clis tree to a fresh temp dir, restoring the swapped
// globals afterwards; it returns the temp tree's root.
func resetAll(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	savedAll, savedRoot := all, userConfigDir
	t.Cleanup(func() { all, userConfigDir = savedAll, savedRoot })
	userConfigDir = dir
	return dir
}

const userCliYAML = "npm:\n  package: mycli\n\nconfig_home_mount: \".mycli\"\n\n" +
	"cmd: \"mycli\"\ncontinue_args: \"-c\"\nresume_args: \"--resume\"\n\nallow_domains:\n  - mycli.dev\n"

// writeUserCli lays out a user cli at dir/clis/<name> like the embedded clis tree:
// CLI.yaml plus optional config files under config/.
func writeUserCli(t *testing.T, dir, name, yamlBody string, configFiles map[string]string) {
	t.Helper()
	root := filepath.Join(dir, "clis", name)
	require.NoError(t, os.MkdirAll(root, ioutil.Dir))
	require.NoError(t, os.WriteFile(filepath.Join(root, "CLI.yaml"), []byte(yamlBody), ioutil.File))
	for p, body := range configFiles {
		dest := filepath.Join(root, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(dest), ioutil.Dir))
		require.NoError(t, os.WriteFile(dest, []byte(body), ioutil.File))
	}
}

func TestLoadUser(t *testing.T) {
	t.Run("merges valid clis into All, in name order", func(t *testing.T) {
		dir := resetAll(t)
		writeUserCli(t, dir, "mycli", userCliYAML, nil)
		all = mustLoadAll()

		names := make([]string, 0, len(all))
		for _, c := range all {
			names = append(names, c.Name)
		}
		assert.Equal(t, []string{"claude", "codex", "grok", "mycli", "opencode"}, names)

		c, ok := For("mycli")
		require.True(t, ok)
		assert.Equal(t, ".mycli", c.ConfigHomeMount)
		assert.Equal(t, []string{"mycli.dev"}, c.AllowDomains)
	})

	t.Run("missing dir loads nothing", func(t *testing.T) {
		resetAll(t)
		all = mustLoadAll()
		assert.Len(t, all, 4)
	})

	t.Run("skips bad clis with a warning", func(t *testing.T) {
		for _, tt := range []struct {
			name    string
			body    string
			configs map[string]string
		}{
			{name: "mycli", body: userCliYAML}, // sanity: a good cli does merge
			{name: "claude", body: userCliYAML},
			{name: "bad", body: "bogus: true\n"},
			{name: "dual", body: "npm:\n  package: x\nversionurl:\n  url: y\n"},
			{name: "filecfg", body: userCliYAML, configs: map[string]string{"config": "junk"}}, // seed config is a file
		} {
			t.Run(tt.name, func(t *testing.T) { testLoadUserSkips(t, tt.name, tt.body, tt.configs) })
		}
	})
}

// testLoadUserSkips loads a single user cli and pins how loading treats it.
func testLoadUserSkips(t *testing.T, name, body string, configs map[string]string) {
	t.Helper()
	dir := resetAll(t)
	writeUserCli(t, dir, name, body, configs)
	all = mustLoadAll()

	switch name {
	case "mycli":
		assert.Contains(t, allNames(), "mycli")
		assert.Len(t, all, 5)
	case "claude":
		claudes := 0
		for _, c := range all {
			if c.Name == "claude" {
				claudes++
			}
		}
		assert.Equal(t, 1, claudes,
			"the embedded cli stays; the conflicting user one is not merged")
		assert.Len(t, all, 4)
	default: // skip cases
		assert.NotContains(t, allNames(), name)
		assert.Len(t, all, 4)
	}
}

// allNames lists every loaded cli's name.
func allNames() []string {
	names := make([]string, 0, len(all))
	for _, c := range all {
		names = append(names, c.Name)
	}
	return names
}

func TestNames(t *testing.T) {
	assert.Equal(t, []string{"claude", "codex", "grok", "opencode"}, Names())
}

// validCliYAML is a valid CLI.yaml, mutated one bad field at a time in parseRejectsTests
const validCliYAML = "npm:\n  package: mycli\nconfig_home_mount: \".mycli\"\ncmd: \"mycli\"\n" +
	"continue_args: \"-c\"\nresume_args: \"--resume\"\n"

func TestParse_Rejects(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want []string
	}{
		{
			"no install source", "cmd: mycli\n",
			[]string{"CLI.OneNotNil", "must have exactly one of [Npm VersionURL] non-nil, got []"},
		},
		{
			"both install sources", "npm:\n  package: mycli\nversionurl:\n  url: https://x\n",
			[]string{"CLI.OneNotNil", "must have exactly one of [Npm VersionURL] non-nil, got [Npm VersionURL]"},
		},
		{
			"empty npm package", "npm:\n  package: \"\"\n",
			[]string{"Npm.Package.Match"},
		},
		{
			"bad npm package", "npm:\n  package: \"My CLI\"\n",
			[]string{"Npm.Package.Match"},
		},
		{
			"partial versionurl urls", "versionurl:\n  url: https://x\n",
			[]string{"LinuxX64URL.Match", "LinuxArm64URL.Match"},
		},
		{
			"non-https versionurl url", "versionurl:\n  url: \"ftp://x\"\n  linux_x64_url: https://x\n  linux_arm64_url: https://x\n",
			[]string{"URL.Match"},
		},
		{
			"missing session args", "npm:\n  package: mycli\ncmd: \"mycli\"\nconfig_home_mount: \".mycli\"\n",
			[]string{"ContinueArgs.Present", "ResumeArgs.Present"},
		},
		{
			"absolute config home mount", "npm:\n  package: mycli\nconfig_home_mount: \"/etc/mycli\"\ncmd: \"mycli\"\n" +
				"continue_args: \"-c\"\nresume_args: \"--resume\"\n",
			[]string{"ConfigHomeMount.Match"},
		},
		{
			"bad config dir env key", validCliYAML + "config_dir_env_key: \"bad-key\"\n",
			[]string{"ConfigDirEnvKey.Match"},
		},
		{
			"bad env key", validCliYAML + "env:\n  bad-key: \"1\"\n",
			[]string{"Env", "Match"},
		},
		{
			"empty env value", validCliYAML + "env:\n  FOO: \"\"\n",
			[]string{"Env", "Present"},
		},
		{
			"bad allow domain", validCliYAML + "allow_domains:\n  - \"https://x.dev\"\n",
			[]string{"AllowDomains", "Match"},
		},
		{
			"absolute data bind key", validCliYAML + "data_binds:\n  \"/etc/auth.json\": \"{}\"\n",
			[]string{"DataBinds", "Match"},
		},
		{
			"dotdot data bind key", validCliYAML + "data_binds:\n  \"../auth.json\": \"{}\"\n",
			[]string{"DataBinds", "Match"},
		},
		{
			"empty data bind key", validCliYAML + "data_binds:\n  \"\": \"{}\"\n",
			[]string{"DataBinds", "Match"},
		},
		{
			"unknown key", "npm:\n  package: mycli\nbogus: true\n",
			[]string{"field bogus not found"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parse("clis/mycli/CLI.yaml", []byte(tt.body))
			require.Error(t, err)
			for _, want := range tt.want {
				assert.ErrorContains(t, err, want)
			}
		})
	}
}

func TestSeedCLIFS(t *testing.T) {
	for _, c := range All() {
		fsys := SeedCLIFS(c.Name)
		assert.Containsf(t, seedPaths(t, fsys), c.SeedAgentsFilename, "shared AGENTS doc renamed into place: %s", c.Name)
	}

	claudeFS := SeedCLIFS("claude") // per-CLI tree rooted at its config dir
	want := []string{"CLAUDE.md", "hooks/secret-tripwire.sh", "settings.json", "statusline.sh"}
	assert.Equal(t, want, seedPaths(t, claudeFS))

	assert.PanicsWithValue(t, `harness: unknown cli "emacs"`, func() { SeedCLIFS("emacs") })
}

func TestSeedCLIFS_UserTree(t *testing.T) {
	dir := resetAll(t)
	writeUserCli(t, dir, "mycli", userCliYAML, map[string]string{"config/settings.toml": "[x]\n"})
	writeUserCli(t, dir, "bare", userCliYAML, nil)
	all = mustLoadAll()

	fsys := SeedCLIFS("mycli")
	assert.Equal(t, []string{"AGENTS.md", "settings.toml"}, seedPaths(t, fsys),
		"shared AGENTS renamed in, user config tree merged")

	// no config tree laid down: SeedCLIFS mkdirs an empty one, so only the shared AGENTS doc lands
	bareFS := SeedCLIFS("bare")
	assert.Equal(t, []string{"AGENTS.md"}, seedPaths(t, bareFS))
}

func seedPaths(t *testing.T, fsys *fsutil.FS) []string {
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
	assert.JSONEq(t, `{"name":"claude","npm":{"package":"@anthropic-ai/claude-code"},"versionurl":null}`, body)

	body, err = MustFor("grok").PkgInfoJSON()
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"grok","npm":null,"versionurl":{"url":"https://x.ai/cli/stable",`+
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
	}
	for _, tt := range tests {
		got := tt.cli.SessionCmd(tt.shell, tt.cont, tt.resume, tt.args)
		assert.Equal(t, tt.want, got, "%s shell=%v cont=%v resume=%v", tt.cli.Name, tt.shell, tt.cont, tt.resume)
	}
}

func TestSeed_AgentsFilename(t *testing.T) {
	assert.Equal(t, "CLAUDE.md", MustFor("claude").SeedAgentsFilename) // yaml override
	assert.Equal(t, "AGENTS.md", MustFor("codex").SeedAgentsFilename)  // parse default
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
}

func TestCLI_CLIDataBinds(t *testing.T) {
	authJSON := "{}"
	assert.Equal(t,
		map[string]*string{".local/share/opencode/auth.json": &authJSON},
		MustFor("opencode").DataBinds)

	for _, name := range []string{"claude", "codex", "grok"} {
		assert.Emptyf(t, MustFor(name).DataBinds, "%s has no data binds", name)
	}
}
