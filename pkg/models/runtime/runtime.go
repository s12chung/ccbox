// Package runtime holds the language runtimes the egress wall knows: the shared
// tooling defaults exclude them, so the domains live here alone.
package runtime

import "slices"

// Runtime is a language runtime and the egress wall domains its toolchain talks to.
type Runtime struct {
	// Name names the runtime; its alias derives from it
	Name string

	// Domains are the runtime's egress wall domains
	Domains []string
}

// runtimes is every known runtime, sorted by name.
var runtimes = []Runtime{
	// Go: vanity imports, module proxy, checksum db, toolchain mirror
	{Name: "go", Domains: []string{"golang.org", "proxy.golang.org", "sum.golang.org", "dl.google.com", "storage.googleapis.com"}},
	// Node: the npm + yarn registries and nodejs.org
	{Name: "node", Domains: []string{"registry.npmjs.org", "registry.yarnpkg.com", "nodejs.org"}},
	// Python: the package index and its file host
	{Name: "python", Domains: []string{"pypi.org", "pythonhosted.org"}},
	// Ruby: gems + from-source tarballs
	{Name: "ruby", Domains: []string{"rubygems.org", "cache.ruby-lang.org"}},
}

// AllRuntimesAlias is the allowlist alias expanding to every runtime's domains
const AllRuntimesAlias = "ccbox-all-runtimes"

// All lists every known runtime, sorted by name
func All() []Runtime { return slices.Clone(runtimes) }

// Alias is the runtime's allowlist alias: ccbox-<name>-runtime
func (r Runtime) Alias() string { return "ccbox-" + r.Name + "-runtime" }

// Aliases lists every runtime's alias plus AllRuntimesAlias, sorted
func Aliases() []string {
	aliases := make([]string, 0, len(runtimes)+1)
	for _, r := range runtimes {
		aliases = append(aliases, r.Alias())
	}
	aliases = append(aliases, AllRuntimesAlias)
	return slices.Sorted(slices.Values(aliases))
}

// AllDomains concatenates every runtime's domains
func AllDomains() []string {
	var domains []string
	for _, r := range runtimes {
		domains = append(domains, r.Domains...)
	}
	return domains
}

// DomainsFor resolves an allowlist alias to its runtime's domains — every
// runtime's for AllRuntimesAlias. ok is false for anything but an alias.
func DomainsFor(alias string) ([]string, bool) {
	for _, r := range runtimes {
		if r.Alias() == alias {
			return slices.Clone(r.Domains), true
		}
	}
	if alias == AllRuntimesAlias {
		return AllDomains(), true
	}
	return nil, false
}
