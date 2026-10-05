// Package provider holds the LLM API providers the egress wall knows: a CLI's
// provider traffic is allowlisted by alias, so the domains live here alone.
//
// NOTE: like pkg/cli — Load() runs once at startup (cmd.rootCmd.PersistentPreRunE),
// after seeding. The built-ins are a compile-time constant, while user providers
// warn+skip, effectively guaranteeing a workable set onwards. It runs before
// cli.Load(), whose user clis may alias user providers in allow_domains.
package provider

import (
	"embed"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"

	"github.com/s12chung/ccbox/ccboxtools/pkg/util/log"
	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/kit/yamlutil"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/fsutil"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/uslice"
)

const (
	providersDir = "providers"
	yamlExt      = ".yaml"
	// userGlob matches the user providers: <name>.yaml files; stray files (README.md) don't
	userGlob = providersDir + "/*" + yamlExt

	// AllProvidersAlias is the allowlist alias expanding to every provider's domains
	AllProvidersAlias = "ccbox-all-providers"
)

// Provider is an LLM API provider and the egress wall domains its API talks to.
type Provider struct {
	// Name names the provider; its alias derives from it. A user provider's name
	// comes from its filename — a name key in the yaml is rejected.
	Name string `yaml:"-"`

	// Domains are the provider's egress wall domains
	Domains []string `yaml:"domains"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[Provider]().
		Validates(firm.RuleMap{
			// the name rides the ccbox-<name>-provider alias and the file name
			"Name":    {rule.Match{Regexp: regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)}},
			"Domains": {rule.Len{Min: 1}, firm.Elems[[]string](firmrule.Domain)},
		}))
}

// builtinProviders is every built-in provider, sorted by name.
var builtinProviders = []Provider{
	{Name: "anthropic", Domains: []string{"api.anthropic.com"}},
	{Name: "openai", Domains: []string{"api.openai.com", "auth.openai.com"}},
	{Name: "openrouter", Domains: []string{"openrouter.ai"}},
	{Name: "xai", Domains: []string{"x.ai"}}, // also hosts grok's own downloads
	{Name: "zai", Domains: []string{"api.z.ai"}},
}

// providers is the loaded provider set: the built-ins until Load() swaps in the
// built-ins overridden by the user's.
var providers = builtinProviders

// Load loads the provider set: the built-in providers, overridden by the user's
func Load() { providers = mustLoadAll() }

// mustLoadAll merges the user providers over the built-ins: a user provider replaces
// a same-name built-in. User load warnings print; the set always loads.
func mustLoadAll() []Provider {
	userProviders, warns, err := LoadUserProviders()
	for _, warn := range warns {
		log.Warnf("%s, skipping", warn)
	}
	// unreachable: listing a dir never errors
	must.Do(err)

	byName := make(map[string]Provider, len(builtinProviders)+len(userProviders))
	for _, p := range slices.Concat(builtinProviders, userProviders) {
		byName[p.Name] = p
	}
	return uslice.Map(slices.Sorted(maps.Keys(byName)), func(name string) Provider { return byName[name] })
}

// All lists every known provider, sorted by name
func All() []Provider { return slices.Clone(providers) }

// Alias is the provider's allowlist alias: ccbox-<name>-provider
func (p Provider) Alias() string { return "ccbox-" + p.Name + "-provider" }

// Aliases lists every provider's alias plus AllProvidersAlias, sorted
func Aliases() []string {
	aliases := make([]string, 0, len(providers)+1)
	for _, p := range providers {
		aliases = append(aliases, p.Alias())
	}
	aliases = append(aliases, AllProvidersAlias)
	return slices.Sorted(slices.Values(aliases))
}

// DomainsFor resolves an allowlist alias to its provider's domains — every
// provider's for AllProvidersAlias. ok is false for anything but an alias.
func DomainsFor(alias string) ([]string, bool) {
	for _, p := range providers {
		if p.Alias() == alias {
			return slices.Clone(p.Domains), true
		}
	}
	if alias == AllProvidersAlias {
		var domains []string
		for _, p := range providers {
			domains = append(domains, p.Domains...)
		}
		return domains, true
	}
	return nil, false
}

// userConfigDir is the user config dir: ~/.ccbox/config.
var userConfigDir = userdir.ConfigDir()

// SetUserConfigDir points the user config dir at dir; tests redirect the user
// providers at a temp tree with it.
func SetUserConfigDir(dir string) { userConfigDir = dir }

// UserDir is the host dir of user-defined providers: ~/.ccbox/config/providers.
func UserDir() string { return filepath.Join(userConfigDir, providersDir) }

// LoadUserProviders parses the user providers tree, each named after its file. A bad
// provider is a warning — skipped with a startup warning; doctor is where they surface.
func LoadUserProviders() ([]Provider, []error, error) {
	return fsutil.LoadGlob(os.DirFS(userConfigDir), userGlob, func(_ string, info fsutil.EntryInfo) (Provider, error) {
		return parseProvider(strings.TrimSuffix(info.Name(), yamlExt), info.Body)
	})
}

// parseProvider decodes one provider yaml body into its Provider, named name —
// firm-validated to fail at load, not on first use.
func parseProvider(name string, body []byte) (Provider, error) {
	p, err := yamlutil.ValidatedDecode(body, func(newProvider Provider) Provider {
		newProvider.Name = name
		return newProvider
	})
	if err != nil {
		return Provider{}, fmt.Errorf("provider: parse %s: %w", name, err)
	}
	return p, nil
}

//go:embed user-providers
var userSeed embed.FS

// UserSeedFS returns the embedded tree seeded onto a fresh UserDir().
func UserSeedFS() fs.FS { return must.Get(fs.Sub(userSeed, "user-providers")) }
