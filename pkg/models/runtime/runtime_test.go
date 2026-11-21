package runtime

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
	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/models/modelset"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/slicex"
)

// containerHome is the image's user home the image state hangs off
const containerHome = "/home/ccbox"

func TestMain(m *testing.M) {
	// ignore any user runtimes on this machine: tests pin the built-in set
	set.LoadBuiltins()
	os.Exit(m.Run())
}

func TestAliases(t *testing.T) {
	aliases := set.Aliases()

	assert.Len(t, slices.Compact(slices.Clone(aliases)), len(aliases), "aliases are unique")
	for _, r := range All() {
		assert.Containsf(t, aliases, r.Alias(), "%s's alias is listed", r.Name)
	}
	assert.Contains(t, aliases, AllRuntimesAlias)
}

func TestNode(t *testing.T) {
	assert.Equal(t, nodeName, Node().Name)
}

// The Dockerfile applies the bootstrap packages only before `mise install` — a
// purge of an AptBuildDeps entry that collides with an AptPkgs lib would ship
// an image missing a runtime lib.
func TestAll_AptBuildDepsDisjointFromAptPkgs(t *testing.T) {
	var pkgs []string
	for _, r := range All() {
		pkgs = append(pkgs, r.AptPkgs...)
	}
	for _, r := range All() {
		for _, dep := range r.AptBuildDeps {
			assert.NotContainsf(t, pkgs, dep, "%s's %s is purged, never reapplied", r.Name, dep)
		}
	}
}

// TestData_Corroborates cross-checks the image-state fields against each other:
// the data is handwritten, so a stale dir should fail here, not the image build.
func TestAll_Data_Corroborates(t *testing.T) {
	for _, r := range All() {
		t.Run(r.Name, func(t *testing.T) {
			dirs := slices.Concat(r.CacheVolumes, r.BinEntries)

			// every env'd path is a cache volume or PATH entry: the volumes seed what the env points at
			for k, v := range r.EnvVars {
				if !strings.HasPrefix(v, containerHome+"/") {
					continue // not per-project state (NPM_CONFIG_UPDATE_NOTIFIER=false, RUSTUP_HOME system-side)
				}
				assert.Containsf(t, dirs, v, "%s points outside the runtime's dirs", k)
			}

			// PATH entries hang off a cache volume: the volume seeds the bin dir
			for _, bin := range r.BinEntries {
				derived := slices.ContainsFunc(r.CacheVolumes, func(v string) bool { return v == bin || v+"/bin" == bin })
				assert.Truef(t, derived, "%s hangs off no cache volume", bin)
			}
		})
	}
}

func TestAll_Domains_Valid(t *testing.T) {
	for _, r := range All() {
		assert.NotEmptyf(t, r.Domains, "%s has domains", r.Name)
		for _, d := range r.Domains {
			assert.Nilf(t, firm.Value[string](firmrule.Domain).Validate(d).ToNil(), "%s: %s", r.Name, d)
		}
	}
}

func TestAllCacheVolumes(t *testing.T) {
	var want []string
	for _, r := range All() {
		want = append(want, r.CacheVolumes...)
	}
	assert.NotEmpty(t, want)
	assert.Equal(t, want, AllCacheVolumes())
}

// TestUserDir pins the drop-ins' user-facing path — the join is modelset's
func TestUserDir(t *testing.T) {
	modelset.SetUserConfigDir("/home/me/.ccbox/config")
	t.Cleanup(func() { modelset.SetUserConfigDir(userdir.ConfigDir()) }) // tests are the only setters
	assert.Equal(t, "/home/me/.ccbox/config/runtimes", Set().UserDir())
}

// resetAll redirects the user runtimes at a fresh temp dir, restoring the
// built-in set afterwards.
func resetAll(t *testing.T) {
	t.Helper()
	modelset.SetUserConfigDir(t.TempDir())
	t.Cleanup(func() {
		set.LoadBuiltins()
		modelset.SetUserConfigDir(userdir.ConfigDir()) // tests are the only setters
	})
}

// userRuntimeYAML is a full-fields user runtime: ruby, the one beyond the built-ins
const userRuntimeYAML = `domains:
  - rubygems.org
mise_tool:
  postinstall_cmd_tmpl: "gem install{{range .}} {{.Name}}{{end}}"
  postinstall_tools:
    - name: rubygems:bundler
mise_tools_optional:
  - name: aqua:rubyfmt
    hint: ruby formatter
env_vars:
  BUNDLE_PATH: /home/ccbox/.bundle
bin_entries:
  - /home/ccbox/.bundle/bin
cache_volumes:
  - /home/ccbox/.bundle
`

// writeUserRuntime writes a user runtime at set.UserDir()/<name>.yaml
func writeUserRuntime(t *testing.T, name, yamlBody string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(set.UserDir(), osutil.Dir))
	require.NoError(t, os.WriteFile(filepath.Join(set.UserDir(), name+".yaml"), []byte(yamlBody), osutil.File))
}

// names lists the loaded set's names
func names() []string {
	return slicex.Map(All(), func(r Runtime) string { return r.Name })
}

// loaded returns the loaded runtime by name — the zero Runtime for an unknown one
func loaded(name string) Runtime {
	for _, r := range All() {
		if r.Name == name {
			return r
		}
	}
	return Runtime{}
}

func TestLoadUser(t *testing.T) {
	// the wiring smoke test — the load mechanics are modelset's, tested on its widget
	t.Run("a drop-in merges in, resolving by its alias", func(t *testing.T) {
		resetAll(t)
		writeUserRuntime(t, "ruby", userRuntimeYAML)
		writeUserRuntime(t, "elixir", userRuntimeYAML)
		Load()

		assert.Equal(t, []string{"elixir", "go", nodeName, "python", "ruby", "rust"}, names())

		domains, ok := set.DomainsFor("ccbox-ruby-runtime")
		require.True(t, ok)
		assert.Equal(t, []string{"rubygems.org"}, domains)
	})

	t.Run("a full-fields runtime lands its image state", func(t *testing.T) {
		resetAll(t)
		writeUserRuntime(t, "ruby", userRuntimeYAML)
		Load()

		ruby := loaded("ruby")
		assert.Equal(t, mise.Tool{
			Name:               "ruby", // mise_tool unnamed: the runtime's own name
			PostInstallCmdTmpl: "gem install{{range .}} {{.Name}}{{end}}",
			PostInstallTools:   []mise.Tool{{Name: "rubygems:bundler"}},
		}, ruby.MiseTool)
		assert.Equal(t, []mise.Tool{{Name: "aqua:rubyfmt", Hint: "ruby formatter"}}, ruby.MiseToolsOptional)
		assert.Equal(t, map[string]string{"BUNDLE_PATH": "/home/ccbox/.bundle"}, ruby.EnvVars)
		assert.Equal(t, []string{"/home/ccbox/.bundle/bin"}, ruby.BinEntries)
		assert.Equal(t, []string{"/home/ccbox/.bundle"}, ruby.CacheVolumes)
	})

	t.Run("an aliased mise tool keeps the runtime's own name", func(t *testing.T) {
		resetAll(t)
		writeUserRuntime(t, "python3", "domains: [pypi.org]\nmise_tool:\n  name: python\n")
		Load()

		python3 := loaded("python3")
		assert.Equal(t, "python3", python3.Name)
		assert.Equal(t, "python", python3.MiseTool.Name)
		assert.Equal(t, "ccbox-python3-runtime", python3.Alias())
	})
}

// TestLoadUser_OverridesBuiltin proves an override reaches Node(), which reads
// the loaded set
func TestLoadUser_OverridesBuiltin(t *testing.T) {
	resetAll(t)
	writeUserRuntime(t, nodeName, "domains:\n  - registry.npmjs.example\n")
	Load()

	assert.Equal(t, []string{"registry.npmjs.example"}, Node().Domains)
}

func TestParseRuntime_Rejects(t *testing.T) {
	for _, tt := range []struct {
		name      string
		parseName string
		body      string
		want      []string
	}{
		{
			"empty body", "", "",
			[]string{"Len", "minimum length"}, // the zero Runtime fails validation
		},
		{
			"no domains", "", "bogus: true\n",
			[]string{"field bogus not found"},
		},
		{
			"name key", "", "name: ruby\ndomains: [rubygems.org]\n",
			[]string{"field name not found"},
		},
		{
			"empty domains", "", "domains: []\n",
			[]string{"Len", "minimum length"},
		},
		{
			"bad domain", "", "domains: [\"https://rubygems.org\"]\n",
			[]string{"Match"},
		},
		{
			"empty domain", "", "domains: [\"\"]\n",
			[]string{"Match"},
		},
		{
			"bad name", "Bad_Name", userRuntimeYAML,
			[]string{"Match"}, // the filename feeds Name
		},
		{
			"bad env key", "", "domains: [rubygems.org]\nenv_vars: {bundle_path: /x}\n",
			[]string{"EnvVar"},
		},
		{
			"optional tool without a name", "", "domains: [rubygems.org]\nmise_tools_optional:\n  - hint: no name\n",
			[]string{"Present"},
		},
		{
			"postinstall tool without a name", "", "domains: [rubygems.org]\nmise_tool:\n  postinstall_tools:\n    - hint: no name\n",
			[]string{"PostInstallTools.[0].Name.Present"},
		},
		{
			"nested postinstall tool without a name", "",
			"domains: [rubygems.org]\nmise_tool:\n  postinstall_tools:\n    - name: a\n      postinstall_tools:\n        - hint: no name\n",
			[]string{"PostInstallTools.[0].PostInstallTools.[0].Name.Present"},
		},
		{
			"unparseable postinstall tmpl", "", "domains: [rubygems.org]\nmise_tool:\n  postinstall_cmd_tmpl: \"{{bogus\"\n",
			[]string{"Template"},
		},
		{
			"relative cache volume", "", "domains: [rubygems.org]\ncache_volumes: [relative]\n",
			[]string{"CacheVolumes", "Match"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			name := tt.parseName
			if name == "" {
				name = "ruby"
			}
			_, err := parseRuntime(name, []byte(tt.body))
			require.Error(t, err)
			assert.NotContains(t, err.Error(), strings.ToLower("panic"))
			require.ErrorContains(t, err, "runtime: parse "+name+": ")
			for _, want := range tt.want {
				assert.ErrorContains(t, err, want)
			}
		})
	}
}
