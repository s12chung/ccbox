package harness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

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

		assert.Equal(t, []string{"claude", "codex", "grok", "mycli", "opencode", "pi"}, Names())

		c, ok := For("mycli")
		require.True(t, ok)
		assert.Equal(t, ".mycli", c.ConfigHomeMount)
		assert.Equal(t, []string{"mycli.dev"}, c.AllowDomains)
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

func TestParse_Rejects(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want []string
	}{
		{
			"no install source", "cmd: mycli\n",
			[]string{"CLI.PkgInfo.OneNotNil", "must have exactly one of [Npm VersionURL] non-nil, got []"},
		},
		{
			"both install sources", "npm:\n  package: mycli\nversion_url:\n  url: https://x\n",
			[]string{"CLI.PkgInfo.OneNotNil", "must have exactly one of [Npm VersionURL] non-nil, got [Npm VersionURL]"},
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
			"partial version_url urls", "version_url:\n  url: https://x\n",
			[]string{"LinuxX64URL.Match", "LinuxArm64URL.Match"},
		},
		{
			"non-https version_url url", "version_url:\n  url: \"ftp://x\"\n  linux_x64_url: https://x\n  linux_arm64_url: https://x\n",
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
