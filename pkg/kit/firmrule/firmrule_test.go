package firmrule

import (
	"testing"

	"github.com/s12chung/firm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type parent struct {
	Npm        *npm
	VersionURL *versionURL
}

type (
	npm        struct{}
	versionURL struct{}
)

func TestDefinedOnce_ValidateValue(t *testing.T) {
	v := firm.Value[parent](DefinedOnce{Fields: []string{"Npm", "VersionURL"}})

	assert.Nil(t, v.Validate(parent{Npm: &npm{}}).ToNil())
	assert.Nil(t, v.Validate(parent{VersionURL: &versionURL{}}).ToNil())

	tests := []struct {
		name string
		data parent
		want string
	}{
		{"none set", parent{}, "want exactly one of [Npm VersionURL], got []"},
		{"both set", parent{Npm: &npm{}, VersionURL: &versionURL{}}, "want exactly one of [Npm VersionURL], got [Npm VersionURL]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMap := v.Validate(tt.data)
			require.Len(t, errMap, 1)
			assert.Contains(t, errMap.Error(), tt.want)
		})
	}
}

func TestDefinedOnce_TypeCheck(t *testing.T) {
	_, err := firm.ValueWithErr[parent](DefinedOnce{Fields: []string{"Missing"}})
	require.ErrorContains(t, err, "has no pointer field, Missing")

	_, err = firm.ValueWithErr[parent](DefinedOnce{Fields: []string{"Npm", "parent"}})
	require.ErrorContains(t, err, "has no pointer field, parent")

	_, err = firm.ValueWithErr[int](DefinedOnce{Fields: []string{"Npm"}})
	require.ErrorContains(t, err, "is not a Struct")
}

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
			[]string{"node_modules", ".idea", "vendor/bundle", "a"},
			[]string{"", "/etc", "../escape", "..", "./x", "."},
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
			"FileName", FileName,
			[]string{"AGENTS.md", "CLAUDE.md", "a", "a-b_c.d"},
			[]string{"", "dir/a.md", ".hidden", "-x", "a b"},
		},
		{
			"NpmPackage", NpmPackage,
			[]string{"opencode-ai", "@anthropic-ai/claude-code", "a", "a.b_c-d"},
			[]string{"", "MyCLI", "foo/bar", "@scope", "-x"},
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
