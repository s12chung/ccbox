package firmrule

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"github.com/s12chung/firm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestDomainOrAlias(t *testing.T) {
	aliases := []string{"ccbox-anthropic-provider", "ccbox-all-providers", "ccbox-defaults"}
	v := firm.Value[string](DomainOrAlias(aliases))
	quoted := make([]string, len(aliases))
	for i, a := range aliases {
		quoted[i] = strconv.Quote(a)
	}

	t.Run("domains and aliases are valid", func(t *testing.T) {
		for _, s := range slices.Concat([]string{"x.ai", "api.anthropic.com"}, aliases) {
			assert.Nilf(t, v.Validate(s).ToNil(), "want %q valid", s)
		}
	})
	t.Run("other values fail with the allowed list", func(t *testing.T) {
		for _, s := range []string{"", "https://x.dev", "X.AI", "ccbox-bogus-provider", "ccbox-set-harness"} {
			errMap := v.Validate(s)
			require.NotEmptyf(t, errMap, "want %q invalid", s)
			assert.Containsf(t, errMap.Error(), "DomainOrAlias: value is not a domain or one of "+fmt.Sprintf("%v", quoted), s)
		}
	})
	t.Run("type checks to a string", func(t *testing.T) {
		assert.Nil(t, DomainOrAlias(aliases).TypeCheck(reflect.TypeFor[string]()))
		assert.NotNil(t, DomainOrAlias(aliases).TypeCheck(reflect.TypeFor[int]()))
	})
}

func TestBind(t *testing.T) {
	valid := []map[string]string{
		{"gitconfig": "enabled"},
		{"~/shared/fonts": "~/fonts"}, // a home-relative mount
		{"~/shared/fonts": "/home/ccbox/fonts"},
		{"/srv/ca": "/home/ccbox/.local/share/ca"},
		{"~/.config": "/dot", "/srv": "/srv"},
	}
	invalid := []struct {
		binds map[string]string
		want  string
	}{
		{map[string]string{"gitconfig": "/home/ccbox/.config/git"}, "must be enabled"}, // no custom mount over the default
		{map[string]string{"gitconfig": ""}, "must be enabled"},
		{map[string]string{"fonts": "/mnt"}, "must be gitconfig or a host path"},
		{map[string]string{"../escape": "/mnt"}, "must be gitconfig or a host path"},
		{map[string]string{"fonts/": "/mnt"}, "must be gitconfig or a host path"},
		{map[string]string{"~/fonts": "enabled"}, "must be a container mount path"},
		{map[string]string{"~/fonts": "mnt"}, "must be a container mount path"},
		{map[string]string{"~/fonts": "/mnt/../x"}, "must be a container mount path"},
	}

	// the pair rule runs per entry, as KeyValues passes it down
	v := firm.KeyValues[map[string]string](Bind{Specials: []string{"gitconfig"}})
	for _, binds := range valid {
		assert.Nilf(t, v.Validate(binds).ToNil(), "want %v valid", binds)
	}
	for _, tt := range invalid {
		errMap := v.Validate(tt.binds)
		require.NotEmptyf(t, errMap, "want %v invalid", tt.binds)
		assert.Containsf(t, errMap.Error(), tt.want, "%v", tt.binds)
		for host := range tt.binds { // the error names the entry
			assert.Containsf(t, errMap.Error(), "["+host+"]", "%v", tt.binds)
		}
	}

	t.Run("specials are injectable", func(t *testing.T) {
		v := firm.KeyValues[map[string]string](Bind{Specials: []string{"cas"}})
		assert.Nil(t, v.Validate(map[string]string{"cas": "enabled"}).ToNil())
		assert.NotEmpty(t, v.Validate(map[string]string{"gitconfig": "enabled"})) // no longer special
	})

	t.Run("type checks to a string map", func(t *testing.T) {
		assert.Nil(t, Bind{}.TypeCheck(reflect.TypeFor[map[string]string]()))
		assert.NotNil(t, Bind{}.TypeCheck(reflect.TypeFor[map[string]int]()))
		assert.NotNil(t, Bind{}.TypeCheck(reflect.TypeOf(true)))
	})
}
