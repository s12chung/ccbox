package modelset

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/slicex"
)

// widget is a test model: a name-derived alias plus domains, mirroring the
// provider and runtime models
type widget struct {
	Name    string   `yaml:"-"`
	Domains []string `yaml:"domains"`
}

func (w widget) Key() string          { return w.Name }
func (w widget) Alias() string        { return "ccbox-" + w.Name + "-widget" }
func (w widget) DomainList() []string { return w.Domains }

const (
	widgetsDir   = "widgets"
	allWidgetAli = "ccbox-all-widgets"
)

func parseWidget(name string, body []byte) (widget, error) {
	var w widget
	if err := yaml.Unmarshal(body, &w); err != nil {
		return widget{}, err
	}
	if len(w.Domains) == 0 {
		return widget{}, errors.New("no domains")
	}
	w.Name = name
	return w, nil
}

func newTestSet(builtins []widget) *AliasedSet[widget] {
	return &AliasedSet[widget]{
		Set:      NewSet(builtins, widgetsDir, parseWidget),
		AllAlias: allWidgetAli,
	}
}

var builtinWidgets = []widget{
	{Name: "alpha", Domains: []string{"alpha.dev"}},
	{Name: "beta", Domains: []string{"api.beta.dev", "auth.beta.dev"}},
}

// resetAll redirects the user config dir at a fresh temp dir, restoring the
// built-in set afterwards.
func resetAll(t *testing.T, set *AliasedSet[widget]) {
	t.Helper()
	SetUserConfigDir(t.TempDir())
	t.Cleanup(func() {
		set.LoadBuiltins()
		SetUserConfigDir(userdir.ConfigDir()) // tests are the only setters
	})
}

// writeUserWidget writes a user widget at UserDir()/<name>.yaml
func writeUserWidget(t *testing.T, set *AliasedSet[widget], name, yamlBody string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(set.UserDir(), osutil.Dir))
	require.NoError(t, os.WriteFile(filepath.Join(set.UserDir(), name+yamlExt), []byte(yamlBody), osutil.File))
}

// names lists the loaded set's names
func names(set *AliasedSet[widget]) []string { return widgetNames(set.All()) }

func TestSet_Load_Merges(t *testing.T) {
	t.Run("valid drop-ins merge in name order", func(t *testing.T) {
		set := newTestSet(builtinWidgets)
		resetAll(t, set)
		writeUserWidget(t, set, "zeta", "domains: [zeta.dev]\n")
		writeUserWidget(t, set, "gamma", "domains: [gamma.dev]\n")
		set.Load()

		assert.Equal(t, []string{"alpha", "beta", "gamma", "zeta"}, names(set))
	})

	t.Run("a drop-in replaces a same-name built-in", func(t *testing.T) {
		set := newTestSet(builtinWidgets)
		resetAll(t, set)
		writeUserWidget(t, set, "alpha", "domains: [api.alpha.example]\n")
		set.Load()

		assert.Equal(t, []string{"alpha", "beta"}, names(set))
		domains, ok := set.DomainsFor("ccbox-alpha-widget")
		require.True(t, ok)
		assert.Equal(t, []string{"api.alpha.example"}, domains)
	})

	t.Run("a bad drop-in is skipped, the set still loads", func(t *testing.T) {
		set := newTestSet(builtinWidgets)
		resetAll(t, set)
		writeUserWidget(t, set, "badyaml", "domains: [\n")
		writeUserWidget(t, set, "gamma", "domains: [gamma.dev]\n")

		set.Load()

		assert.Equal(t, []string{"alpha", "beta", "gamma"}, names(set))
	})

	t.Run("stray files and a missing dir load the built-ins only", func(t *testing.T) {
		set := newTestSet(builtinWidgets)
		resetAll(t, set)

		set.Load() // no widgets dir at all
		assert.Equal(t, []string{"alpha", "beta"}, names(set))

		require.NoError(t, os.MkdirAll(set.UserDir(), osutil.Dir))
		require.NoError(t, os.WriteFile(filepath.Join(set.UserDir(), "README.md"), []byte("junk"), osutil.File))
		set.Load()
		assert.Equal(t, []string{"alpha", "beta"}, names(set))
	})
}

func TestSet_LoadUser(t *testing.T) {
	t.Run("parses each drop-in, named after its file", func(t *testing.T) {
		set := newTestSet(builtinWidgets)
		resetAll(t, set)
		writeUserWidget(t, set, "zeta", "domains: [zeta.dev]\n")

		userWidgets, warns, err := set.LoadUser()

		require.NoError(t, err)
		assert.Empty(t, warns)
		assert.Equal(t, []string{"zeta"}, widgetNames(userWidgets))
	})

	t.Run("a bad drop-in is a warning, skipped from the tree", func(t *testing.T) {
		set := newTestSet(builtinWidgets)
		resetAll(t, set)
		writeUserWidget(t, set, "badyaml", "domains: [\n")
		writeUserWidget(t, set, "nodomains", "domains: []\n")
		writeUserWidget(t, set, "gamma", "domains: [gamma.dev]\n")

		userWidgets, warns, err := set.LoadUser()

		require.NoError(t, err)
		assert.Equal(t, []string{"gamma"}, widgetNames(userWidgets))
		assert.Len(t, warns, 2)
	})
}

func TestSet_LoadBuiltins(t *testing.T) {
	set := newTestSet(builtinWidgets)
	resetAll(t, set)
	writeUserWidget(t, set, "zeta", "domains: [zeta.dev]\n")
	set.Load()
	set.LoadBuiltins()

	assert.Equal(t, []string{"alpha", "beta"}, names(set))
}

func TestSet_All_Clones(t *testing.T) {
	set := newTestSet(builtinWidgets)
	all := set.All()
	all[0].Name = "mutated"

	assert.Equal(t, "alpha", set.All()[0].Name)
}

func TestSet_UserDir(t *testing.T) {
	set := newTestSet(builtinWidgets)
	SetUserConfigDir("/home/me/.ccbox/config")
	t.Cleanup(func() { SetUserConfigDir(userdir.ConfigDir()) }) // tests are the only setters
	assert.Equal(t, "/home/me/.ccbox/config", UserConfigDir())
	assert.Equal(t, "/home/me/.ccbox/config/widgets", set.UserDir())
}

func TestAliasedSet_Aliases(t *testing.T) {
	set := newTestSet(builtinWidgets)

	aliases := set.Aliases()

	assert.Equal(t, []string{"ccbox-all-widgets", "ccbox-alpha-widget", "ccbox-beta-widget"}, aliases)
}

func TestAliasedSet_AllDomains(t *testing.T) {
	set := newTestSet(builtinWidgets)

	assert.Equal(t, []string{"alpha.dev", "api.beta.dev", "auth.beta.dev"}, set.AllDomains())
}

func TestAliasedSet_DomainsFor(t *testing.T) {
	set := newTestSet(builtinWidgets)

	t.Run("a model's alias resolves its domains, cloned", func(t *testing.T) {
		domains, ok := set.DomainsFor("ccbox-beta-widget")
		require.True(t, ok)
		assert.Equal(t, []string{"api.beta.dev", "auth.beta.dev"}, domains)

		domains[0] = "mutated.dev"
		again, _ := set.DomainsFor("ccbox-beta-widget")
		assert.Equal(t, "api.beta.dev", again[0])
	})

	t.Run("the all alias resolves every model's domains", func(t *testing.T) {
		domains, ok := set.DomainsFor(allWidgetAli)
		require.True(t, ok)
		assert.Equal(t, set.AllDomains(), domains)
	})

	t.Run("a non-alias does not resolve", func(t *testing.T) {
		for _, alias := range []string{"alpha.dev", "ccbox-bogus-widget", "ccbox-all-providers", ""} {
			_, ok := set.DomainsFor(alias)
			assert.Falsef(t, ok, "%q is no alias", alias)
		}
	})
}

// widgetNames lists the widgets' names
func widgetNames(widgets []widget) []string {
	return slicex.Map(widgets, func(w widget) string { return w.Name })
}
