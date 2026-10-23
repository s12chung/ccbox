// Package runtime holds the language runtimes the image knows: the egress wall
// domains their toolchains talk to, plus everything baked about them — the mise
// tools, apt libs, env, PATH entries and cache mounts `ccbox build` composes the
// mise config (pkg/mise), the Dockerfile's runtime-owned regions (pkg/dockerfile)
// and dmap's per-project cache volumes from.
package runtime

import (
	"slices"
	"strings"

	"github.com/s12chung/ccbox/pkg/mise"
)

// Runtime is a language runtime and everything the image knows about it: the
// egress wall domains its toolchain talks to, and the image state the build
// composes from.
type Runtime struct {
	Name    string   // the alias derives from it
	Domains []string // the egress wall domains

	MiseTool          mise.Tool   // the runtime itself, as the mise config installs it
	MiseToolsOptional []mise.Tool // the ecosystem tooling installed with the runtime

	AptPkgs      []string          // shared libs the mise config's [bootstrap.packages] installs
	AptBuildDeps []string          // the from-source compile's headers, purged right after
	EnvVars      map[string]string // the baked env — consumers sort the keys
	EnvBuildDeps map[string]string // from-source install's env, exported to the install RUN

	BinEntries   []string // the container PATH additions
	CacheVolumes []string // the container dirs dmap maps per-project cache volumes to; the name's suffix is path.Base minus its leading dot
}

// runtimes is every selectable runtime, sorted by name — node is not selectable
// (see node below).
var runtimes = []Runtime{
	// Go: vanity imports, module proxy, checksum db, toolchain mirror
	{
		Name:              "go",
		Domains:           []string{"golang.org", "proxy.golang.org", "sum.golang.org", "dl.google.com", "storage.googleapis.com"},
		MiseTool:          mise.Tool{Name: "go"},
		MiseToolsOptional: []mise.Tool{{Name: "golangci-lint"}},
		EnvVars:           map[string]string{"GOBIN": "/home/ccbox/go/bin"}, // mise sets its own GOBIN
		BinEntries:        []string{"/home/ccbox/go/bin"},
		CacheVolumes:      []string{"/home/ccbox/go"}, // go mod tidy module cache + GOBIN
	},
	// Python: the package index and its file host
	{
		Name:    "python",
		Domains: []string{"pypi.org", "pythonhosted.org"},
		// CPython 3.12+ dropped setuptools/wheel; the postinstall reinstalls them
		MiseTool: mise.Tool{
			Name:               "python",
			PostInstallCmdTmpl: `python -m pip install --no-cache-dir{{range .}} {{.Name}}=={{.Version}}{{end}}`,
			PostInstallTools:   []mise.Tool{{Name: "pypi:setuptools"}, {Name: "pypi:wheel"}},
		},
		MiseToolsOptional: []mise.Tool{{Name: "aqua:astral-sh/ruff"}, {Name: "aqua:pypa/pipx"}},
	},
	// Ruby: gems + from-source tarballs
	{
		Name:         "ruby",
		Domains:      []string{"rubygems.org", "cache.ruby-lang.org"},
		MiseTool:     mise.Tool{Name: "ruby"},
		AptPkgs:      []string{"libssl3t64", "libyaml-0-2", "zlib1g", "libffi8", "libreadline8t64", "libgmp10", "libzstd1"},
		AptBuildDeps: []string{"libssl-dev", "libyaml-dev", "zlib1g-dev", "libffi-dev", "libreadline-dev", "libgmp-dev"},
		EnvVars:      map[string]string{"GEM_HOME": "/home/ccbox/.gem"},
		BinEntries:   []string{"/home/ccbox/.gem/bin"},
		CacheVolumes: []string{"/home/ccbox/.gem"}, // bundler GEM_HOME
	},
	// Rust: rustup-init + the toolchain dists it installs, the crates index and its downloads
	{
		Name:     "rust",
		Domains:  []string{"sh.rustup.rs", "static.rust-lang.org", "crates.io", "index.crates.io", "static.crates.io"},
		MiseTool: mise.Tool{Name: "rust"},
		// no MiseToolsOptional: clippy and rustfmt ship as the toolchain's rustup
		// components
		// The build's HOME (root's) is unreadable to ccbox, and rustup hangs its
		// state off it by default — pin both homes system-side, baking only the
		// toolchain home the proxies resolve through
		EnvVars: map[string]string{"RUSTUP_HOME": "/usr/local/share/mise/rustup"},
		EnvBuildDeps: map[string]string{
			"CARGO_HOME":  "/usr/local/share/mise/cargo",
			"RUSTUP_HOME": "/usr/local/share/mise/rustup",
		},
		BinEntries:   []string{"/home/ccbox/.cargo/bin"},
		CacheVolumes: []string{"/home/ccbox/.cargo"}, // cargo registry + git dep caches; cargo install's bin dir
	},
}

// node is installed by default — the devbox tooling depends on it.
var node = Runtime{
	Name:              "node",
	Domains:           []string{"registry.npmjs.org", "registry.yarnpkg.com", "nodejs.org"},
	MiseTool:          mise.Tool{Name: "node", Hint: "required"},
	MiseToolsOptional: []mise.Tool{{Name: "npm:oxlint"}, {Name: "npm:oxfmt"}},
	AptPkgs:           []string{"libatomic1"}, // V8 needs libatomic on arm64
	EnvVars: map[string]string{
		"NPM_CONFIG_PREFIX":          "/home/ccbox/.npm-global",
		"NPM_CONFIG_UPDATE_NOTIFIER": "false",
	},
	BinEntries:   []string{"/home/ccbox/.npm-global/bin"},
	CacheVolumes: []string{"/home/ccbox/.npm", "/home/ccbox/.npm-global"}, // npm download cache; global packages
}

// AllRuntimesAlias is the allowlist alias expanding to every runtime's domains
const AllRuntimesAlias = "ccbox-all-runtimes"

// All lists every runtime — node plus the selectable ones, sorted by name
func All() []Runtime {
	all := append(slices.Clone(runtimes), node)
	slices.SortFunc(all, func(a, b Runtime) int { return strings.Compare(a.Name, b.Name) })
	return all
}

// Alias is the runtime's allowlist alias: ccbox-<name>-runtime
func (r Runtime) Alias() string { return "ccbox-" + r.Name + "-runtime" }

// Node is the always-installed runtime: the prov spec's runtimes never list it.
func Node() Runtime { return node }

// Aliases lists every runtime's alias plus AllRuntimesAlias, sorted
func Aliases() []string {
	all := All()
	aliases := make([]string, 0, len(all)+1)
	for _, r := range all {
		aliases = append(aliases, r.Alias())
	}
	aliases = append(aliases, AllRuntimesAlias)
	return slices.Sorted(slices.Values(aliases))
}

// AllDomains concatenates every runtime's domains
func AllDomains() []string {
	var domains []string
	for _, r := range All() {
		domains = append(domains, r.Domains...)
	}
	return domains
}

// AllCacheVolumes concatenates every runtime's cache volumes
func AllCacheVolumes() []string {
	var volumes []string
	for _, r := range All() {
		volumes = append(volumes, r.CacheVolumes...)
	}
	return volumes
}

// DomainsFor resolves an allowlist alias to its runtime's domains — every
// runtime's for AllRuntimesAlias. ok is false for anything but an alias.
func DomainsFor(alias string) ([]string, bool) {
	for _, r := range All() {
		if r.Alias() == alias {
			return slices.Clone(r.Domains), true
		}
	}
	if alias == AllRuntimesAlias {
		return AllDomains(), true
	}
	return nil, false
}
