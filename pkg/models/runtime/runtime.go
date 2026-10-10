// Package runtime holds the language runtimes the image knows: the egress wall
// domains their toolchains talk to, plus everything baked about them — the mise
// tools, apt libs, env, PATH entries and cache mounts `ccbox build` composes the
// mise config (pkg/mise), the Dockerfile's runtime-owned regions (pkg/dockerfile)
// and dmap's per-project cache volumes from.
package runtime

import (
	"slices"

	"github.com/s12chung/ccbox/pkg/mise"
)

// Runtime is a language runtime and everything the image knows about it: the
// egress wall domains its toolchain talks to, and the image state the build
// composes from.
type Runtime struct {
	// Name names the runtime; its alias derives from it
	Name string

	// Domains are the runtime's egress wall domains
	Domains []string

	// MiseTool is the runtime itself, as the mise config installs it
	MiseTool mise.Tool

	// MiseToolsOptional groups the ecosystem tooling riding the runtime's install —
	// a grouping, not a condition: every entry installs with the rest
	MiseToolsOptional []mise.Tool

	// AptPkgs are the runtime's shared libs kept in the image — present because of
	// this runtime, not always exclusively (libssl3t64, zlib1g also serve libcurl)
	AptPkgs []string

	// AptBuildDeps are build headers for the mise install's from-source compile,
	// purged right after
	AptBuildDeps []string

	// CacheVolumes are the container dirs dmap maps per-project cache volumes to;
	// the volume name's suffix is path.Base minus its leading dot
	CacheVolumes []string

	// BinEntries are the runtime's container PATH additions
	BinEntries []string

	// EnvVars are the runtime's image env (GOBIN, GEM_HOME, NPM_CONFIG_*). Go map
	// iteration is randomized — consumers sort the keys.
	EnvVars map[string]string
}

// runtimes is every known runtime, sorted by name.
var runtimes = []Runtime{
	// Go: vanity imports, module proxy, checksum db, toolchain mirror
	{
		Name:              "go",
		Domains:           []string{"golang.org", "proxy.golang.org", "sum.golang.org", "dl.google.com", "storage.googleapis.com"},
		MiseTool:          mise.Tool{Tool: "go", Version: "1.26"},
		MiseToolsOptional: []mise.Tool{{Tool: "golangci-lint", Version: "2.13.1"}},
		CacheVolumes:      []string{"/home/ccbox/go"}, // go mod tidy module cache + GOBIN
		BinEntries:        []string{"/home/ccbox/go/bin"},
		EnvVars:           map[string]string{"GOBIN": "/home/ccbox/go/bin"},
	},
	// Node: the npm + yarn registries and nodejs.org
	{
		Name:         "node",
		Domains:      []string{"registry.npmjs.org", "registry.yarnpkg.com", "nodejs.org"},
		MiseTool:     mise.Tool{Tool: "node", Version: "26.3.0"},
		AptPkgs:      []string{"libatomic1"},                                  // V8 needs libatomic on arm64
		CacheVolumes: []string{"/home/ccbox/.npm", "/home/ccbox/.npm-global"}, // npm download cache; global packages
		BinEntries:   []string{"/home/ccbox/.npm-global/bin"},
		EnvVars: map[string]string{
			"NPM_CONFIG_PREFIX":          "/home/ccbox/.npm-global",
			"NPM_CONFIG_UPDATE_NOTIFIER": "false",
		},
	},
	// Python: the package index and its file host
	{
		Name:    "python",
		Domains: []string{"pypi.org", "pythonhosted.org"},
		// postinstall re-adds setuptools/wheel CPython 3.12+ dropped (setuptools = 3.13's distutils shim)
		MiseTool: mise.Tool{
			Tool:        "python",
			Version:     "3.13",
			PostInstall: "python -m pip install --no-cache-dir setuptools==82.0.1 wheel==0.47.0",
		},
	},
	// Ruby: gems + from-source tarballs
	{
		Name:         "ruby",
		Domains:      []string{"rubygems.org", "cache.ruby-lang.org"},
		MiseTool:     mise.Tool{Tool: "ruby", Version: "3.4"},
		AptPkgs:      []string{"libssl3t64", "libyaml-0-2", "zlib1g", "libffi8", "libreadline8t64", "libgmp10", "libzstd1"},
		AptBuildDeps: []string{"libssl-dev", "libyaml-dev", "zlib1g-dev", "libffi-dev", "libreadline-dev", "libgmp-dev"},
		CacheVolumes: []string{"/home/ccbox/.gem"}, // bundler GEM_HOME
		BinEntries:   []string{"/home/ccbox/.gem/bin"},
		EnvVars:      map[string]string{"GEM_HOME": "/home/ccbox/.gem"},
	},
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

// AllMiseTools flattens every runtime's tools in All's order — the mise config's
// [tools] input
func AllMiseTools() []mise.Tool {
	var tools []mise.Tool
	for _, r := range runtimes {
		tools = append(tools, r.MiseTool)
		tools = append(tools, r.MiseToolsOptional...)
	}
	return tools
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
