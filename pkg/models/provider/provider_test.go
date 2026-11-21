package provider

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/s12chung/firm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/models/modelset"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/slicex"
)

func TestMain(m *testing.M) {
	// ignore any user providers on this machine: tests pin the built-in set
	set.LoadBuiltins()
	os.Exit(m.Run())
}

func TestAliases(t *testing.T) {
	aliases := set.Aliases()

	assert.Len(t, slices.Compact(slices.Clone(aliases)), len(aliases), "aliases are unique")
	for _, p := range set.All() {
		assert.Containsf(t, aliases, p.Alias(), "%s's alias is listed", p.Name)
	}
	assert.Contains(t, aliases, AllProvidersAlias)
}

func TestDomains_Valid(t *testing.T) {
	for _, p := range builtinProviders {
		assert.NotEmptyf(t, p.Domains, "%s has domains", p.Name)
		for _, d := range p.Domains {
			assert.Nilf(t, firm.Value[string](firmrule.Domain).Validate(d).ToNil(), "%s: %s", p.Name, d)
		}
	}
}

// TestUserDir pins the drop-ins' user-facing path — the join is modelset's
func TestUserDir(t *testing.T) {
	modelset.SetUserConfigDir("/home/me/.ccbox/config")
	t.Cleanup(func() { modelset.SetUserConfigDir(userdir.ConfigDir()) }) // tests are the only setters
	assert.Equal(t, "/home/me/.ccbox/config/providers", Set().UserDir())
}

// resetAll redirects the user providers at a fresh temp dir, restoring the
// built-in set afterwards.
func resetAll(t *testing.T) {
	t.Helper()
	modelset.SetUserConfigDir(t.TempDir())
	t.Cleanup(func() {
		set.LoadBuiltins()
		modelset.SetUserConfigDir(userdir.ConfigDir()) // tests are the only setters
	})
}

const userProviderYAML = "domains:\n  - api.myprovider.dev\n"

// writeUserProvider writes a user provider at set.UserDir()/<name>.yaml
func writeUserProvider(t *testing.T, name, yamlBody string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(set.UserDir(), osutil.Dir))
	require.NoError(t, os.WriteFile(filepath.Join(set.UserDir(), name+".yaml"), []byte(yamlBody), osutil.File))
}

// names lists the loaded set's names
func names() []string {
	return slicex.Map(set.All(), func(p Provider) string { return p.Name })
}

// TestLoadUser pins the wiring — the load mechanics are modelset's, tested on
// its widget: a drop-in merges in and resolves by its alias.
func TestLoadUser(t *testing.T) {
	resetAll(t)
	writeUserProvider(t, "myprovider", userProviderYAML)
	writeUserProvider(t, "another", userProviderYAML)
	Load()

	assert.Equal(t, []string{"another", "anthropic", "google", "myprovider", "openai", "openrouter", "xai", "zai"}, names())

	domains, ok := set.DomainsFor("ccbox-myprovider-provider")
	require.True(t, ok)
	assert.Equal(t, []string{"api.myprovider.dev"}, domains)
}

func TestParseProvider_Rejects(t *testing.T) {
	for _, tt := range []struct {
		name      string
		parseName string
		body      string
		want      []string
	}{
		{
			"empty body", "", "",
			[]string{"Len", "minimum length"}, // the zero Provider fails validation
		},
		{
			"no domains", "", "bogus: true\n",
			[]string{"field bogus not found"},
		},
		{
			"name key", "", "name: myprovider\ndomains: [x.dev]\n",
			[]string{"field name not found"},
		},
		{
			"empty domains", "", "domains: []\n",
			[]string{"Len", "minimum length"},
		},
		{
			"bad domain", "", "domains: [\"https://x.dev\"]\n",
			[]string{"Match"},
		},
		{
			"empty domain", "", "domains: [\"\"]\n",
			[]string{"Match"},
		},
		{
			"bad name", "Bad_Name", userProviderYAML,
			[]string{"Match"}, // the filename feeds Name
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			name := tt.parseName
			if name == "" {
				name = "myprovider"
			}
			_, err := parseProvider(name, []byte(tt.body))
			require.Error(t, err)
			assert.NotContains(t, err.Error(), strings.ToLower("panic"))
			require.ErrorContains(t, err, "provider: parse "+name+": ")
			for _, want := range tt.want {
				assert.ErrorContains(t, err, want)
			}
		})
	}
}
