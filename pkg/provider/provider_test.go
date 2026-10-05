package provider

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/s12chung/firm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/uslice"
)

func TestMain(m *testing.M) {
	// ignore any user providers on this machine: tests pin the built-in set
	providers = slices.Clone(builtinProviders)
	os.Exit(m.Run())
}

func TestAliases(t *testing.T) {
	aliases := Aliases()

	assert.True(t, slices.IsSorted(aliases))
	assert.Len(t, slices.Compact(slices.Clone(aliases)), len(aliases), "aliases are unique")
	for _, p := range providers {
		assert.Containsf(t, aliases, p.Alias(), "%s's alias is listed", p.Name)
	}
	assert.Contains(t, aliases, AllProvidersAlias)
}

func TestAll(t *testing.T) {
	names := make([]string, 0, len(providers))
	for _, p := range All() {
		names = append(names, p.Name)
		assert.NotEmptyf(t, p.Domains, "%s has domains", p.Name)
	}
	assert.True(t, slices.IsSorted(names))
}

func TestDomainsFor(t *testing.T) {
	for _, provider := range providers {
		domains, ok := DomainsFor(provider.Alias())
		assert.Truef(t, ok, "%s's alias resolves", provider.Name)
		assert.Equal(t, provider.Domains, domains)
	}

	t.Run(AllProvidersAlias+" resolves every provider's domains", func(t *testing.T) {
		domains, ok := DomainsFor(AllProvidersAlias)
		assert.True(t, ok)
		for _, p := range providers {
			for _, d := range p.Domains {
				assert.Containsf(t, domains, d, "%s: %s", p.Name, d)
			}
		}
	})

	t.Run("a non-alias does not resolve", func(t *testing.T) {
		for _, alias := range []string{"api.anthropic.com", "ccbox-bogus-provider", "ccbox-defaults", ""} {
			_, ok := DomainsFor(alias)
			assert.Falsef(t, ok, "%q is no alias", alias)
		}
	})
}

func TestDomains_Valid(t *testing.T) {
	for _, p := range providers {
		for _, d := range p.Domains {
			assert.Nilf(t, firm.Value[string](firmrule.Domain).Validate(d).ToNil(), "%s: %s", p.Name, d)
		}
	}
}

func TestUserDir(t *testing.T) {
	SetUserConfigDir("/home/me/.ccbox/config")
	t.Cleanup(func() { SetUserConfigDir(userdir.ConfigDir()) }) // tests are the only setters
	assert.Equal(t, "/home/me/.ccbox/config/providers", UserDir())
}

// resetAll redirects the user providers at a fresh temp dir, restoring the swapped
// globals afterwards.
func resetAll(t *testing.T) {
	t.Helper()
	SetUserConfigDir(t.TempDir())
	savedProviders := providers
	t.Cleanup(func() {
		providers = savedProviders
		SetUserConfigDir(userdir.ConfigDir()) // tests are the only setters
	})
}

const userProviderYAML = "domains:\n  - api.myprovider.dev\n"

// writeUserProvider writes a user provider at UserDir()/<name>.yaml
func writeUserProvider(t *testing.T, name, yamlBody string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(UserDir(), ioutil.Dir))
	require.NoError(t, os.WriteFile(filepath.Join(UserDir(), name+yamlExt), []byte(yamlBody), ioutil.File))
}

// names lists the loaded set's names
func names() []string {
	return uslice.Map(All(), func(p Provider) string { return p.Name })
}

func TestLoadUser(t *testing.T) {
	t.Run("merges valid providers into All, in name order", func(t *testing.T) {
		resetAll(t)
		writeUserProvider(t, "myprovider", userProviderYAML)
		writeUserProvider(t, "another", userProviderYAML)
		Load()

		assert.Equal(t, []string{"another", "anthropic", "myprovider", "openai", "openrouter", "xai", "zai"}, names())

		domains, ok := DomainsFor("ccbox-myprovider-provider")
		require.True(t, ok)
		assert.Equal(t, []string{"api.myprovider.dev"}, domains)
	})

	t.Run("missing dir loads the built-ins only", func(t *testing.T) {
		resetAll(t)
		Load()
		assert.Equal(t, uslice.Map(builtinProviders, func(p Provider) string { return p.Name }), names())
	})

	t.Run("stray files are ignored", func(t *testing.T) {
		resetAll(t)
		require.NoError(t, os.MkdirAll(UserDir(), ioutil.Dir))
		require.NoError(t, os.WriteFile(filepath.Join(UserDir(), "README.md"), []byte("drop-ins live here"), ioutil.File))
		require.NoError(t, os.WriteFile(filepath.Join(UserDir(), "notes.txt"), []byte("junk"), ioutil.File))
		Load()

		assert.Equal(t, uslice.Map(builtinProviders, func(p Provider) string { return p.Name }), names())
	})

	t.Run("skips bad providers with a warning", func(t *testing.T) {
		for _, tt := range []struct {
			filename string
			body     string
		}{
			{filename: "good", body: userProviderYAML}, // sanity: a good provider merges
			{filename: "badyaml", body: "domains: ["},
			{filename: "unknownfield", body: "bogus: true\n"},
			{filename: "namekey", body: "name: myprovider\ndomains: [api.myprovider.dev]\n"},
			{filename: "emptybody", body: ""},
			{filename: "emptydomains", body: "domains: []\n"},
			{filename: "baddomain", body: "domains: [\"https://x.dev\"]\n"},
			{filename: "Bad_Name", body: userProviderYAML},
		} {
			t.Run(tt.filename, func(t *testing.T) { testLoadUserSkips(t, tt.filename, tt.body) })
		}
	})
}

// testLoadUserSkips loads a single user provider and pins how loading treats it.
func testLoadUserSkips(t *testing.T, filename, body string) {
	t.Helper()
	resetAll(t)
	writeUserProvider(t, filename, body)
	Load()

	wantNames := uslice.Map(builtinProviders, func(p Provider) string { return p.Name })
	if filename == "good" {
		wantNames = append(wantNames, filename)
		slices.Sort(wantNames)
	}
	assert.Equal(t, wantNames, names())
}

func TestLoadUserProviders_Warns(t *testing.T) {
	resetAll(t)
	writeUserProvider(t, "bad", "bogus: true\n")

	userProviders, warns, err := LoadUserProviders()

	require.NoError(t, err)
	assert.Empty(t, userProviders)
	require.Len(t, warns, 1)
	assert.ErrorContains(t, warns[0], "provider: parse bad: ")
}

func TestLoadUser_OverridesBuiltin(t *testing.T) {
	resetAll(t)
	writeUserProvider(t, "anthropic", "domains:\n  - api.anthropic.example\n")
	Load()

	// map keys are unique: the user provider replaces the built-in, not appends to it
	assert.Len(t, All(), len(builtinProviders))

	domains, ok := DomainsFor("ccbox-anthropic-provider")
	require.True(t, ok)
	assert.Equal(t, []string{"api.anthropic.example"}, domains)
}

func TestParseProvider_Rejects(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
		want []string
	}{
		{
			"empty body", "",
			[]string{"EOF"},
		},
		{
			"no domains", "bogus: true\n",
			[]string{"field bogus not found"},
		},
		{
			"name key", "name: myprovider\ndomains: [x.dev]\n",
			[]string{"field name not found"},
		},
		{
			"empty domains", "domains: []\n",
			[]string{"Len", "minimum length"},
		},
		{
			"bad domain", "domains: [\"https://x.dev\"]\n",
			[]string{"Match"},
		},
		{
			"empty domain", "domains: [\"\"]\n",
			[]string{"Match"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseProvider("myprovider", []byte(tt.body))
			require.Error(t, err)
			assert.NotContains(t, err.Error(), strings.ToLower("panic"))
			for _, want := range tt.want {
				assert.ErrorContains(t, err, want)
			}
		})
	}
}

func TestUserSeedFS(t *testing.T) {
	var paths []string
	require.NoError(t, fs.WalkDir(UserSeedFS(), ".", func(p string, d fs.DirEntry, err error) error {
		require.NoError(t, err)
		if !d.IsDir() {
			paths = append(paths, p)
		}
		return nil
	}))
	assert.Equal(t, []string{"README.md"}, paths)
}
