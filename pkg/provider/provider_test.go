package provider

import (
	"slices"
	"testing"

	"github.com/s12chung/firm"
	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/pkg/kit/firmrule"
)

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
