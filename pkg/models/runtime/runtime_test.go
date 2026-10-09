package runtime

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
	for _, r := range runtimes {
		assert.Containsf(t, aliases, r.Alias(), "%s's alias is listed", r.Name)
	}
	assert.Contains(t, aliases, AllRuntimesAlias)
}

func TestAll(t *testing.T) {
	names := make([]string, 0, len(runtimes))
	for _, r := range All() {
		names = append(names, r.Name)
		assert.NotEmptyf(t, r.Domains, "%s has domains", r.Name)
	}
	assert.True(t, slices.IsSorted(names))
}

func TestDomainsFor(t *testing.T) {
	for _, runtime := range runtimes {
		domains, ok := DomainsFor(runtime.Alias())
		assert.Truef(t, ok, "%s's alias resolves", runtime.Name)
		assert.Equal(t, runtime.Domains, domains)
	}

	t.Run(AllRuntimesAlias+" resolves every runtime's domains", func(t *testing.T) {
		domains, ok := DomainsFor(AllRuntimesAlias)
		assert.True(t, ok)
		assert.Equal(t, AllDomains(), domains)
	})

	t.Run("a non-alias does not resolve", func(t *testing.T) {
		for _, alias := range []string{"rubygems.org", "ccbox-bogus-runtime", "ccbox-defaults", "ccbox-all-providers", ""} {
			_, ok := DomainsFor(alias)
			assert.Falsef(t, ok, "%q is no alias", alias)
		}
	})
}

func TestDomains_Valid(t *testing.T) {
	for _, r := range runtimes {
		for _, d := range r.Domains {
			assert.Nilf(t, firm.Value[string](firmrule.Domain).Validate(d).ToNil(), "%s: %s", r.Name, d)
		}
	}
}
