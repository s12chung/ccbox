package runtime

import (
	"slices"
	"strings"
	"testing"

	"github.com/s12chung/firm"
	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/pkg/kit/firmrule"
)

// containerHome is the image's user home the image state hangs off
const containerHome = "/home/ccbox"

func TestAliases(t *testing.T) {
	aliases := Aliases()

	assert.True(t, slices.IsSorted(aliases))
	assert.Len(t, slices.Compact(slices.Clone(aliases)), len(aliases), "aliases are unique")
	for _, r := range runtimes {
		assert.Containsf(t, aliases, r.Alias(), "%s's alias is listed", r.Name)
	}
	assert.Contains(t, aliases, AllRuntimesAlias)
}

func TestNode(t *testing.T) {
	assert.Equal(t, "node", Node().Name)
}

func TestAll(t *testing.T) {
	names := make([]string, 0, len(runtimes))
	for _, r := range All() {
		names = append(names, r.Name)
		assert.NotEmptyf(t, r.Domains, "%s has domains", r.Name)
	}
	assert.True(t, slices.IsSorted(names))
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

func TestAllCacheVolumes(t *testing.T) {
	var want []string
	for _, r := range All() {
		want = append(want, r.CacheVolumes...)
	}
	assert.NotEmpty(t, want)
	assert.Equal(t, want, AllCacheVolumes())
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

// TestData_Corroborates cross-checks the image-state fields against each other:
// the data is handwritten, so a stale dir should fail here, not the image build.
func TestData_Corroborates(t *testing.T) {
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

func TestDomains_Valid(t *testing.T) {
	for _, r := range runtimes {
		for _, d := range r.Domains {
			assert.Nilf(t, firm.Value[string](firmrule.Domain).Validate(d).ToNil(), "%s: %s", r.Name, d)
		}
	}
}
