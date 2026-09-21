package firmrule

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/s12chung/firm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

func TestMatchRules(t *testing.T) {
	tests := []struct {
		name    string
		rule    firm.Rule
		valid   []string
		invalid []string
	}{
		{
			"Domain", Domain,
			[]string{"x.ai", "githubusercontent.com", "api.anthropic.com", "ccbox-defaults"},
			[]string{"", "https://x.dev", "X.AI", "x..ai", "x.ai/", "*.x.ai"},
		},
		{
			"EnvVar", EnvVar,
			[]string{"CLAUDE_CONFIG_DIR", "_FOO", "A"},
			[]string{"", "1FOO", "bad-key", "FOO BAR"},
		},
		{
			"MaskDir", MaskDir,
			[]string{"node_modules", ".idea", "vendor/bundle", "a", "a/.venv", ".local/share/opencode/auth.json"},
			[]string{"", "/etc", "../escape", "..", "./x", ".", "a/..", "a//b", "a/", "x/..", "~/x", "a/~"},
		},
		{
			"MaskGlob", MaskGlob,
			[]string{"secrets", ".ccbox.yaml", ".env.*", "*.pem", "**/*.pem", "dist/**", "**", "*", "a/b/c.txt", "a/.hidden"},
			[]string{"", "/etc", "../escape", "..", "dist/../x", "a/..", "./x", "a/", "//x", "x//y"},
		},
		{
			"HomePath", HomePath,
			[]string{".claude", ".config/opencode", "cli"},
			[]string{"", "/etc/cli", "/"},
		},
		{
			"HTTPSURL", HTTPSURL,
			[]string{"https://x.ai/cli/stable", "https://x.ai/cli/grok-$version-linux-x86_64"},
			[]string{"", "http://x", "ftp://x", "https://", "x.ai/cli"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := firm.Value[string](tt.rule)
			for _, s := range tt.valid {
				assert.Nilf(t, v.Validate(s).ToNil(), "want %q valid", s)
			}
			for _, s := range tt.invalid {
				errMap := v.Validate(s)
				require.NotEmptyf(t, errMap, "want %q invalid", s)
				assert.Contains(t, errMap.Error(), "Match")
			}
		})
	}
}

func TestHasValidGitDir(t *testing.T) {
	v := firm.Value[bool](HasValidGitDir{})

	t.Run("false passes without resolving the dir", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "") // would error if the rule resolved the dir

		assert.Nil(t, v.Validate(false).ToNil())
	})

	t.Run("true passes when the host git dir resolves", func(t *testing.T) {
		xdg := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", xdg)
		require.NoError(t, os.MkdirAll(filepath.Join(xdg, "git"), ioutil.Dir))

		assert.Nil(t, v.Validate(true).ToNil())
	})

	t.Run("true errors when the host git dir is unresolvable", func(t *testing.T) {
		file := filepath.Join(t.TempDir(), "not-a-dir")
		require.NoError(t, os.WriteFile(file, nil, ioutil.File))
		t.Setenv("XDG_CONFIG_HOME", file) // a file, not a dir: stat <file>/git errors

		errMap := v.Validate(true)
		require.NotEmpty(t, errMap)
		assert.Contains(t, errMap.Error(), "HasValidGitDir")
		assert.Contains(t, errMap.Error(), "not a directory")
	})
}
