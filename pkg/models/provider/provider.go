// Package provider holds the LLM API providers the egress wall knows: a CLI's
// provider traffic is allowlisted by alias, so the domains live here alone.
package provider

import (
	"fmt"

	"github.com/s12chung/firm"

	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/kit/yamlkit"
	"github.com/s12chung/ccbox/pkg/models/modelset"
)

// Load loads the provider set: the built-ins, overridden by the user's drop-ins
func Load() { set.Load() }

const (
	providersDir = "providers"
	// AllProvidersAlias is the allowlist alias expanding to every provider's domains
	AllProvidersAlias = "ccbox-all-providers"
)

var set = &modelset.AliasedSet[Provider]{
	Set:      modelset.NewSet(builtinProviders, providersDir, parseProvider),
	AllAlias: AllProvidersAlias,
}

// Set is the provider set
func Set() *modelset.AliasedSet[Provider] { return set }

// Provider is an LLM API provider and the egress wall domains its API talks to.
type Provider struct {
	// Name names the provider; its alias derives from it. A user provider's name
	// comes from its filename — a name key in the yaml is rejected.
	Name string `yaml:"-"`
	// Domains are the provider's egress wall domains
	Domains []string `yaml:"domains"`
}

// Key is the provider's override key — its name
func (p Provider) Key() string { return p.Name }

// Alias is the provider's allowlist alias
func (p Provider) Alias() string { return "ccbox-" + p.Name + "-provider" }

// DomainList is the provider's egress wall domains
func (p Provider) DomainList() []string { return p.Domains }

func init() {
	firm.MustRegisterType(firm.NewDefinition[Provider]().
		Validates(firm.RuleMap{
			"Name":    {firmrule.Name},
			"Domains": firmrule.Domains,
		}))
}

// builtinProviders is every built-in provider, sorted by name.
var builtinProviders = []Provider{
	{Name: "anthropic", Domains: []string{"api.anthropic.com"}},
	{Name: "google", Domains: []string{
		"accounts.google.com",
		"aicode.googleapis.com",
		"aiplatform.googleapis.com",
		"businessaicode.googleapis.com",
		"cloudcode-pa.googleapis.com",
		"generativelanguage.googleapis.com",
		"oauth2.googleapis.com",
	}},
	{Name: "openai", Domains: []string{"api.openai.com", "auth.openai.com"}},
	{Name: "openrouter", Domains: []string{"openrouter.ai"}},
	{Name: "xai", Domains: []string{"x.ai"}},
	{Name: "zai", Domains: []string{"api.z.ai"}},
}

// parseProvider decodes one provider yaml body into its Provider, named name
func parseProvider(name string, body []byte) (Provider, error) {
	p, err := yamlkit.ValidatedDecode(body, func(newProvider Provider) Provider {
		newProvider.Name = name
		return newProvider
	})
	if err != nil {
		return Provider{}, fmt.Errorf("provider: parse %s: %w", name, err)
	}
	return p, nil
}
