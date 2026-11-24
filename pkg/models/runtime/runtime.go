// Package runtime holds the language runtimes the image knows: the egress wall
// domains their toolchains talk to, and the image state `ccbox build` composes
// the mise config, Dockerfile and cache volumes from.
package runtime

import (
	"fmt"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"

	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/kit/yamlkit"
	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/models/modelset"
)

// Load loads the runtime set: the built-ins, overridden by the user's drop-ins
func Load() { set.Load() }

const (
	runtimesDir = "runtimes"
	nodeName    = "node"
	// AllRuntimesAlias is the allowlist alias expanding to every runtime's domains
	AllRuntimesAlias = "ccbox-all-runtimes"
)

var set = &modelset.AliasedSet[Runtime]{
	Set:      modelset.NewSet(builtinRuntimes, runtimesDir, parseRuntime),
	AllAlias: AllRuntimesAlias,
}

// Set is the runtime set
func Set() *modelset.AliasedSet[Runtime] { return set }

// Runtime is a language runtime and everything the image knows about it: the
// egress wall domains its toolchain talks to, and the image state the build
// composes from.
type Runtime struct {
	// Name names the runtime; its alias derives from it. A user runtime's name
	// comes from its filename — a name key in the yaml is rejected.
	Name string `yaml:"-"`

	// Domains are the runtime's egress wall domains
	Domains []string `yaml:"domains"`

	// MiseTool is the runtime itself, as the mise config installs it
	MiseTool mise.Tool `yaml:"mise_tool"`

	// MiseToolsOptional is the ecosystem tooling installed with the runtime
	MiseToolsOptional []mise.Tool `yaml:"mise_tools_optional"`

	// AptPkgs are the shared libs the mise config's [bootstrap.packages] installs
	AptPkgs []string `yaml:"apt_pkgs"`
	// AptBuildDeps are the from-source compile's headers, purged right after
	AptBuildDeps []string `yaml:"apt_build_deps"`
	// EnvVars is the baked env — consumers sort the keys
	EnvVars map[string]string `yaml:"env_vars"`
	// EnvBuildDeps is the from-source install's env, exported to the install RUN
	EnvBuildDeps map[string]string `yaml:"env_build_deps"`

	// BinEntries are the container PATH additions
	BinEntries []string `yaml:"bin_entries"`

	// CacheVolumes is the container dirs dmap maps per-project cache volumes
	// to; the name's suffix is path.Base minus its leading dot
	CacheVolumes []string `yaml:"cache_volumes"`
}

// Key is the runtime's override key — its name
func (r Runtime) Key() string { return r.Name }

// Alias is the runtime's allowlist alias
func (r Runtime) Alias() string { return "ccbox-" + r.Name + "-runtime" }

// DomainList is the runtime's egress wall domains
func (r Runtime) DomainList() []string { return r.Domains }

func init() {
	firm.MustRegisterType(firm.NewDefinition[Runtime]().
		Validates(firm.RuleMap{
			"Name":              {firmrule.Name},
			"Domains":           firmrule.Domains,
			"MiseTool":          {firm.Backed()},
			"MiseToolsOptional": {firm.Elems[[]mise.Tool](firm.Backed())},
			// apt names are left loose: a bad one fails the image build loudly, at apt
			"EnvVars":      {firm.Keys[map[string]string](firmrule.EnvVar)},
			"EnvBuildDeps": {firm.Keys[map[string]string](firmrule.EnvVar)},
			"BinEntries":   {firm.Elems[[]string](firmrule.ContainerPath)},
			"CacheVolumes": {firm.Elems[[]string](firmrule.ContainerPath)},
		}))
	firm.MustRegisterType(firm.NewDefinition[mise.Tool]().
		Validates(firm.RuleMap{
			"Name":               {rule.Present{}},
			"PostInstallCmdTmpl": {firmrule.Template{}},
			"PostInstallTools":   {firm.Elems[[]mise.Tool](firm.Backed())},
		}))
}

// builtinRuntimes is every built-in runtime, sorted by name.
var builtinRuntimes = []Runtime{
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
	// node is installed by default — the devbox tooling depends on it
	{
		Name:              nodeName,
		Domains:           []string{"registry.npmjs.org", "registry.yarnpkg.com", "nodejs.org"},
		MiseTool:          mise.Tool{Name: nodeName, Hint: "required"},
		MiseToolsOptional: []mise.Tool{{Name: "npm:oxlint"}, {Name: "npm:oxfmt"}},
		AptPkgs:           []string{"libatomic1"}, // V8 needs libatomic on arm64
		EnvVars: map[string]string{
			"NPM_CONFIG_PREFIX":          "/home/ccbox/.npm-global",
			"NPM_CONFIG_UPDATE_NOTIFIER": "false",
		},
		BinEntries:   []string{"/home/ccbox/.npm-global/bin"},
		CacheVolumes: []string{"/home/ccbox/.npm", "/home/ccbox/.npm-global"}, // npm download cache; global packages
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
	// Rust: rustup-init + the toolchain dists it installs, the crates index and its downloads
	{
		Name:     "rust",
		Domains:  []string{"sh.rustup.rs", "static.rust-lang.org", "crates.io", "index.crates.io", "static.crates.io"},
		MiseTool: mise.Tool{Name: "rust"},
		// no MiseToolsOptional: clippy and rustfmt ship as the toolchain's rustup components
		// rustup and cargo default their homes under the build's HOME — root's, unreadable
		// to ccbox — so both are system-side, and only the toolchain home bakes
		EnvVars: map[string]string{"RUSTUP_HOME": "/usr/local/share/mise/rustup"},
		EnvBuildDeps: map[string]string{
			"CARGO_HOME":  "/usr/local/share/mise/cargo",
			"RUSTUP_HOME": "/usr/local/share/mise/rustup",
		},
		BinEntries:   []string{"/home/ccbox/.cargo/bin"},
		CacheVolumes: []string{"/home/ccbox/.cargo"}, // cargo registry + git dep caches; cargo install's bin dir
	},
}

// All lists every known runtime — node is always among them
func All() []Runtime { return set.All() }

// Node is the always-installed runtime — the devbox tooling runs on it.
func Node() Runtime {
	for _, r := range All() {
		if r.Name == nodeName {
			return r
		}
	}
	panic("runtime: node is a built-in, always in the loaded set") // unreachable
}

// AllCacheVolumes concatenates every runtime's cache volumes
func AllCacheVolumes() []string {
	var volumes []string
	for _, r := range All() {
		volumes = append(volumes, r.CacheVolumes...)
	}
	return volumes
}

// parseRuntime decodes one runtime yaml body into its Runtime, named name
func parseRuntime(name string, body []byte) (Runtime, error) {
	r, err := yamlkit.ValidatedDecode(body, func(newRuntime Runtime) Runtime {
		newRuntime.Name = name
		if newRuntime.MiseTool.Name == "" {
			newRuntime.MiseTool.Name = name
		}
		return newRuntime
	})
	if err != nil {
		return Runtime{}, fmt.Errorf("runtime: parse %s: %w", name, err)
	}
	return r, nil
}
