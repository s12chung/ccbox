// Package projectcfg loads the per-project .ccbox.yaml from a workspace repo root
package projectcfg

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/mergeempty"
	"github.com/s12chung/ccbox/pkg/perm"
)

const fileName = ".ccbox.yaml"

// localFileName is for git-ignored override merged onto fileName: lists append, env overlays.
const localFileName = ".ccbox.local.yaml"

// DefaultsToken listed in allowlist, expands in place to allowDefaults
const DefaultsToken = "ccbox-defaults"

// defaultCliName is the coding CLI when .ccbox.yaml doesn't say.
const defaultCliName = harness.NameClaude

// tmpfsDefaults always-masked dirs, prepended only when present in the workspace (to prevent host creation)
var tmpfsDefaults = []string{".idea", ".vscode"}

// volumeDefaults for persistent volume masked dirs only when present in the workspace (to prevent host creation)
var volumeDefaults = []string{"node_modules", ".venv", "vendor/bundle"}

// allowDefaults are the egress domains DefaultsToken expands to: the wall's built-in
// allow — shared defaults plus every supported CLI's own domains.
var allowDefaults = append(append([]string{}, sharedAllowDefaults...), cliAllowDomains()...)

// sharedAllowDefaults are the CLI-independent egress domains.
var sharedAllowDefaults = []string{
	// mise (tool version manager): version lists + release metadata
	"mise.en.dev",
	"mise-versions.jdx.dev",

	// Node / npm
	"registry.npmjs.org",
	"registry.yarnpkg.com",
	"nodejs.org",

	// Playwright browser binaries (image bakes the system libs; browsers fetched per-project)
	"cdn.playwright.dev",
	"playwright.download.prss.microsoft.com",

	// Python
	"pypi.org",
	"pythonhosted.org",

	// Ruby (gems + from-source tarballs)
	"rubygems.org",
	"cache.ruby-lang.org",

	// Go (vanity imports, module proxy, checksum db, toolchain mirror)
	"golang.org",
	"proxy.golang.org",
	"sum.golang.org",
	"dl.google.com",
	"storage.googleapis.com",

	// GitHub: source + release assets (used by gh, delta, yq, rg, fd, jq, python-build-standalone, ruby-build)
	"github.com",
	"githubusercontent.com",
	"githubassets.com",

	// Man pages (canonical man text, not cheatsheets)
	"manpages.debian.org",
	"man7.org",
	"man.cx",
	"linux.die.net",
	"manpages.ubuntu.com",
}

// cliAllowDomains concatenates every supported CLI's own egress domains, in All's order.
func cliAllowDomains() []string {
	var domains []string
	for _, c := range harness.All() {
		domains = append(domains, c.AllowDomains...)
	}
	return domains
}

// Config is the parsed .ccbox.yaml.
type Config struct {
	CLI           harness.Name      `yaml:"cli"`             // coding CLI to install + launch (default claude)
	HostGitConfig *bool             `yaml:"host_git_config"` // read-only mount host ~/.config/git; nil = default on, resolved by Defaulted
	Tmpfs         []string          `yaml:"tmpfs"`           // workspace-relative dirs to mask with a writable tmpfs
	Volumes       []string          `yaml:"volumes"`         // workspace-relative dirs to mask with a persistent per-project volume
	Env           map[string]string `yaml:"env"`             // extra env vars set in the container
	Allowlist     []string          `yaml:"allowlist"`       // egress wall domains; "ccbox-defaults" expands to the built-ins
}

// Defaulted resolves c against its workspace. Applied once by Load.
func Defaulted(workspaceDir string, c Config) Config {
	c.Tmpfs = append(presentDirs(workspaceDir, tmpfsDefaults), c.Tmpfs...)
	c.Volumes = append(presentDirs(workspaceDir, volumeDefaults), c.Volumes...)
	c.Allowlist = expandAllowlist(c.Allowlist)
	if c.CLI == "" {
		c.CLI = defaultCliName
	}
	if c.HostGitConfig == nil { // resolve the default-on so the effective config prints it
		on := true
		c.HostGitConfig = &on
	}
	return c
}

// validate rejects an unknown cli, the one field whose value must be a known enum.
func (c Config) validate() error {
	if _, ok := harness.For(c.CLI); !ok {
		var names []string
		for _, cli := range harness.All() {
			names = append(names, string(cli.Name))
		}
		return fmt.Errorf("cli: unknown value %q (want %q)", c.CLI, strings.Join(names, ", "))
	}
	return nil
}

// MaskDefaults are the built-in dirs masked when present in the workspace: tmpfs then volume.
// Exposed so callers can spot a run creating one that future runs will start masking.
func MaskDefaults() []string {
	return append(append([]string{}, tmpfsDefaults...), volumeDefaults...)
}

// VolumeCleanupDirs is every mask dir whose volume may exist: the built-in defaults (regardless of
// presence) plus explicit config volumes.
func (c Config) VolumeCleanupDirs() []string {
	seen := map[string]bool{}
	var dirs []string
	for _, d := range append(append([]string{}, volumeDefaults...), c.Volumes...) {
		if !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	return dirs
}

// presentDirs returns the entries of dirs that exist as directories under workspaceDir.
func presentDirs(workspaceDir string, dirs []string) []string {
	var out []string
	for _, d := range dirs {
		if info, err := os.Stat(filepath.Join(workspaceDir, d)); err == nil && info.IsDir() {
			out = append(out, d)
		}
	}
	return out
}

// expandAllowlist replaces each DefaultsToken with allowDefaults. A nil list (allowlist
// unset) falls back to the built-ins; an explicit empty list ([]) stays empty, so the wall
// allows nothing.
func expandAllowlist(domains []string) []string {
	if domains == nil {
		domains = []string{DefaultsToken}
	}
	var out []string
	for _, d := range domains {
		if d == DefaultsToken {
			out = append(out, allowDefaults...)
			continue
		}
		out = append(out, d)
	}
	return out
}

// merge layers other onto c and returns a fresh Config
func (c Config) merge(other Config) Config {
	c.Tmpfs = mergeempty.Slice(c.Tmpfs, other.Tmpfs)
	c.Volumes = mergeempty.Slice(c.Volumes, other.Volumes)
	c.Allowlist = mergeempty.Slice(c.Allowlist, other.Allowlist)
	c.Env = mergeempty.Map(c.Env, other.Env)
	if other.CLI != "" { // scalar override: a set local value wins
		c.CLI = other.CLI
	}
	if other.HostGitConfig != nil { // scalar override: a set local value wins
		c.HostGitConfig = other.HostGitConfig
	}
	return c
}

// read parses the .ccbox.yaml at path. A missing file yields the zero Config
func read(path string) (Config, error) {
	body, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := yaml.Unmarshal(body, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Load reads workspaceDir/.ccbox.yaml, layers .ccbox.local.yaml onto it, and
// applies Defaulted. Both files are optional — absent ones contribute the zero Config, so repos
// without either keep working.
func Load(workspaceDir string) (Config, error) {
	base, err := read(filepath.Join(workspaceDir, fileName))
	if err != nil {
		return Config{}, err
	}
	local, err := read(filepath.Join(workspaceDir, localFileName))
	if err != nil {
		return Config{}, err
	}
	c := Defaulted(workspaceDir, base.merge(local))
	if err := c.validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// defaultYAML is the starter .ccbox.yaml
//
//go:embed default.ccbox.yaml
var defaultYAML string

// Init writes defaultYAML to workspaceDir/.ccbox.yaml and returns its path.
// It refuses to clobber an existing file.
func Init(workspaceDir string) (string, error) {
	path := filepath.Join(workspaceDir, fileName)
	switch _, err := os.Stat(path); {
	case err == nil:
		return "", fmt.Errorf("%s already exists", path)
	case !errors.Is(err, fs.ErrNotExist):
		return "", err
	}
	return path, os.WriteFile(path, []byte(defaultYAML), perm.File)
}
