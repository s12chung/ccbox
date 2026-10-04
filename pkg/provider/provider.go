// Package provider holds the LLM API providers the egress wall knows: a CLI's
// provider traffic is allowlisted by alias, so the domains live here alone.
package provider

import "slices"

// Provider is an LLM API provider and the egress wall domains its API talks to.
type Provider struct {
	// Name names the provider; its alias derives from it
	Name string

	// Domains are the provider's egress wall domains
	Domains []string
}

// providers is every known provider, sorted by name.
var providers = []Provider{
	{Name: "anthropic", Domains: []string{"api.anthropic.com"}},
	{Name: "openai", Domains: []string{"api.openai.com", "auth.openai.com"}},
	{Name: "openrouter", Domains: []string{"openrouter.ai"}},
	{Name: "xai", Domains: []string{"x.ai"}}, // also hosts grok's own downloads
	{Name: "zai", Domains: []string{"api.z.ai"}},
}

// AllProvidersAlias is the allowlist alias expanding to every provider's domains
const AllProvidersAlias = "ccbox-all-providers"

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
