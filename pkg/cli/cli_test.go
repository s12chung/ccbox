package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/cli/clitmpl"
	"github.com/s12chung/ccbox/pkg/provider"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestMain(m *testing.M) {
	// ignore any user clis on this machine: tests pin the embedded set
	embedClis := must.Get(wrapTreeLoad(clitmpl.Load(clitmpl.EmbedTree(), parseCLI)))
	all = make(map[string]CLI, len(embedClis))
	for _, c := range embedClis {
		all[c.Name] = c
	}
	os.Exit(m.Run())
}

func TestNames(t *testing.T) {
	assert.Equal(t, []string{"claude", "codex", "grok", "opencode", "pi"}, Names())
}

func TestUserConfigDir(t *testing.T) {
	testutil.FakeHome(t, "/home/me")
	assert.Equal(t, "/home/me/.ccbox/claude", UserConfigDir("claude"))
}

// resetAll redirects the user clis tree to a fresh temp dir, restoring the swapped
// globals afterwards; it returns the temp tree's root.
func resetAll(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	savedAll := all
	t.Cleanup(func() {
		all = savedAll
		clitmpl.SetUserConfigDir(userdir.ConfigDir()) // tests are the only setters
	})
	clitmpl.SetUserConfigDir(dir)
	return dir
}

const userCliYAML = "npm:\n  package: mycli\n\nconfig_home_mount: \".mycli\"\n\n" +
	"cmd: \"mycli\"\ncontinue_args: \"-c\"\nresume_args: \"--resume\"\n\nallow_domains:\n  - mycli.dev\n"

// writeUserCli writes a user cli at dir/clis/<name> like the embedded clis tree:
// CLI.yaml plus optional config files under config/.
func writeUserCli(t *testing.T, dir, name, yamlBody string, configFiles map[string]string) {
	t.Helper()
	root := filepath.Join(dir, "clis", name)
	require.NoError(t, os.MkdirAll(root, osutil.Dir))
	require.NoError(t, os.WriteFile(filepath.Join(root, "CLI.yaml"), []byte(yamlBody), osutil.File))
	for p, body := range configFiles {
		dest := filepath.Join(root, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(dest), osutil.Dir))
		require.NoError(t, os.WriteFile(dest, []byte(body), osutil.File))
	}
}

func TestLoadUser(t *testing.T) {
	t.Run("merges valid clis into All, in name order", func(t *testing.T) {
		dir := resetAll(t)
		writeUserCli(t, dir, "mycli", userCliYAML, nil)
		all = mustLoadAll()

		assert.Equal(t, []string{"claude", "codex", "grok", "mycli", "opencode", "pi"}, Names())

		c, ok := For("mycli")
		require.True(t, ok)
		assert.Equal(t, ".mycli", c.ConfigHomeMount)
		assert.Equal(t, []string{"mycli.dev"}, c.AllowDomains)
		assert.True(t, c.IsUserDefined())
		assert.False(t, MustFor("claude").IsUserDefined())
	})

	t.Run("missing dir loads nothing", func(t *testing.T) {
		resetAll(t)
		all = mustLoadAll()
		assert.Len(t, all, 5)
	})

	t.Run("skips bad clis with a warning", func(t *testing.T) {
		for _, tt := range []struct {
			name    string
			body    string
			configs map[string]string
		}{
			{name: "mycli", body: userCliYAML}, // sanity: a good cli does merge
			{name: "bad", body: "bogus: true\n"},
			{name: "dual", body: "npm:\n  package: x\nversion_url:\n  url: y\n"},
			{name: "filecfg", body: userCliYAML, configs: map[string]string{"config": "junk"}}, // seed config is a file
		} {
			t.Run(tt.name, func(t *testing.T) { testLoadUserSkips(t, tt.name, tt.body, tt.configs) })
		}
	})

	// provider.Load() precedes cli.Load() (cmd.root): a user cli may alias user providers
	t.Run("user provider aliases validate in allow_domains", func(t *testing.T) {
		dir := resetAll(t)
		writeUserCli(t, dir, "mycli", userCliYAML+"  - ccbox-myprovider-provider\n", nil)
		resetUserProviders(t, dir)

		all = mustLoadAll()
		assert.Equal(t, []string{"mycli.dev", "ccbox-myprovider-provider"}, MustFor("mycli").AllowDomains)
	})
}

// resetUserProviders points the user providers at dir/providers, writes a myprovider
// there and loads it — restoring the bare built-ins afterwards, so no state leaks.
func resetUserProviders(t *testing.T, dir string) {
	t.Helper()
	t.Cleanup(func() {
		provider.SetUserConfigDir(t.TempDir())
		provider.Load()
	})
	provider.SetUserConfigDir(dir)
	require.NoError(t, os.MkdirAll(provider.UserDir(), osutil.Dir))
	require.NoError(t, os.WriteFile(filepath.Join(provider.UserDir(), "myprovider.yaml"), []byte("domains: [api.myprovider.dev]\n"), osutil.File))
	provider.Load()
}

func TestLoadUser_OverridesEmbedded(t *testing.T) {
	dir := resetAll(t)
	writeUserCli(t, dir, "claude", userCliYAML, nil)
	all = mustLoadAll()

	// map keys are unique: the user cli replaces the embedded one, not appends to it
	assert.Len(t, all, 5)
	assert.Contains(t, Names(), "claude")

	got, ok := For("claude")
	require.True(t, ok)
	assert.Equal(t, ".mycli", got.ConfigHomeMount, "the user cli's spec wins")
	assert.Equal(t, []string{"mycli.dev"}, got.AllowDomains)
	assert.True(t, got.IsUserDefined())
}

// testLoadUserSkips loads a single user cli and pins how loading treats it.
func testLoadUserSkips(t *testing.T, name, body string, configs map[string]string) {
	t.Helper()
	dir := resetAll(t)
	writeUserCli(t, dir, name, body, configs)
	all = mustLoadAll()

	switch name {
	case "mycli":
		assert.Contains(t, Names(), "mycli")
		assert.Len(t, all, 6)
	default: // skip cases
		assert.NotContains(t, Names(), name)
		assert.Len(t, all, 5)
	}
}

// validCliYAML is a valid CLI.yaml, mutated one bad field at a time in parseRejectsTests
const validCliYAML = "npm:\n  package: mycli\nconfig_home_mount: \".mycli\"\ncmd: \"mycli\"\n" +
	"continue_args: \"-c\"\nresume_args: \"--resume\"\n"

// TestParseCLI_ReleaseURLDoc loads a jq_schema version_url: the selector
// must parse at load, not on first install.
func TestParseCLI_ReleaseURLDoc(t *testing.T) {
	body := "version_url:\n" +
		"  url: \"https://zcode.z.ai/api/v1/releases/electron/manifest\"\n" +
		"  jq_schema:\n" +
		"    format: yaml\n" +
		"    version: \".version\"\n" +
		"    download_url: \".url\"\n" +
		"config_home_mount: \".mycli\"\n" +
		"cmd: \"mycli\"\n" +
		"continue_args: \"-c\"\n" +
		"resume_args: \"--resume\"\n"

	c, err := parseCLI("mycli", []byte(body), false)
	require.NoError(t, err)
	rel := c.ReleaseURL
	assert.Equal(t, "yaml", rel.JQSchema.Format)
	assert.Equal(t, ".version", rel.JQSchema.Version)
}

func TestParseCLI_Rejects(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want []string
	}{
		{
			"no install source", "cmd: mycli\n",
			[]string{"CLI.PkgInfo.OneNotNil", "must have exactly one of [Npm ReleaseURL] non-nil, got []"},
		},
		{
			"both install sources", "npm:\n  package: mycli\nversion_url:\n  url: https://x\n",
			[]string{"CLI.PkgInfo.OneNotNil", "must have exactly one of [Npm ReleaseURL] non-nil, got [Npm ReleaseURL]"},
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
			"no download source", "version_url:\n  url: https://x\n",
			[]string{"ReleaseURL.OneNotNil", "must have exactly one of [DownloadTemplate JQSchema] non-nil, got []"},
		},
		{
			"jq schema on template mode", "version_url:\n  url: https://x\n  download_template:\n    x64_url: https://x\n    arm64_url: https://x\n" +
				"  jq_schema:\n    format: yaml\n    version: \".version\"\n    download_url: \".url\"\n",
			[]string{"ReleaseURL.OneNotNil", "must have exactly one of [DownloadTemplate JQSchema] non-nil, got [DownloadTemplate JQSchema]"},
		},
		{
			"non-https version_url url", "version_url:\n  url: \"ftp://x\"\n  download_template:\n    x64_url: https://x\n    arm64_url: https://x\n",
			[]string{"URL.Match"},
		},
		{
			"non-https download_template", "version_url:\n  url: https://x\n  download_template:\n    x64_url: \"ftp://x\"\n    arm64_url: https://x\n",
			[]string{"X64URL.Match"},
		},
		{
			"bad jq format", "version_url:\n  url: https://x\n" +
				"  jq_schema:\n    format: xml\n    version: \".version\"\n    download_url: \".url\"\n",
			[]string{"Format.OneOf"},
		},
		{
			"bad jq selector", "version_url:\n  url: https://x\n" +
				"  jq_schema:\n    format: yaml\n    version: \".version]\"\n    download_url: \".url\"\n",
			[]string{"JQExpr"},
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
			[]string{"AllowDomains", "DomainOrAlias"},
		},
		{
			"expansion alias in allow_domains", validCliYAML + "allow_domains:\n  - ccbox-defaults\n",
			[]string{"AllowDomains", "DomainOrAlias", "is not a domain or one of " + fmt.Sprintf("%v", quoted(provider.Aliases()))},
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
			_, err := parseCLI("mycli", []byte(tt.body), false)
			require.Error(t, err)
			for _, want := range tt.want {
				assert.ErrorContains(t, err, want)
			}
		})
	}
}

// TestParseCLI_AllowDomainsAliases parses a CLI.yaml whose allow_domains carry the
// provider aliases: they validate like domains, expanding later at the config layer.
func TestParseCLI_AllowDomainsAliases(t *testing.T) {
	body := validCliYAML + "allow_domains:\n  - mycli.dev\n  - ccbox-anthropic-provider\n  - ccbox-all-providers\n"

	c, err := parseCLI("mycli", []byte(body), false)
	require.NoError(t, err)
	assert.Equal(t, []string{"mycli.dev", "ccbox-anthropic-provider", "ccbox-all-providers"}, c.AllowDomains)
}

// quoted renders each string like firm's OneOf error does
func quoted(strs []string) []string {
	out := make([]string, 0, len(strs))
	for _, s := range strs {
		out = append(out, strconv.Quote(s))
	}
	return out
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
	assert.PanicsWithValue(t, `cli: unknown cli "emacs"`, func() { MustFor("emacs") })
}

func TestCLI_PkgInfoJSON(t *testing.T) {
	// the CLI_PKGINFO env JSON each CLI's install source renders
	body, err := MustFor("claude").PkgInfoJSON()
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"claude","npm":{"package":"@anthropic-ai/claude-code"},"version_url":null}`, body)

	body, err = MustFor("grok").PkgInfoJSON()
	require.NoError(t, err)
	assert.JSONEq(t, `{"name":"grok","npm":null,"version_url":{"url":"https://x.ai/cli/stable",`+
		`"download_template":{"x64_url":"https://x.ai/cli/grok-$version-linux-x86_64",`+
		`"arm64_url":"https://x.ai/cli/grok-$version-linux-aarch64"},"jq_schema":null,"artifact":null}}`, body)
}

func TestCLI_SessionCmd(t *testing.T) {
	tests := []struct {
		cli    CLI
		cont   bool
		resume bool
		args   []string
		want   []string
	}{
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
		got := tt.cli.SessionCmd(tt.cont, tt.resume, tt.args)
		assert.Equal(t, tt.want, got, "%s cont=%v resume=%v", tt.cli.Name, tt.cont, tt.resume)
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
