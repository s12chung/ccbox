// Package projectcfg loads the per-project .ccbox.yaml from a workspace repo root
package projectcfg

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/pkg/perm"
)

const fileName = ".ccbox.yaml"

// DefaultsToken listed in allowlist, expands in place to allowDefaults
const DefaultsToken = "ccbox-defaults"

// tmpfsDefaults always-masked dirs, prepended only when present in the workspace (to prevent host creation)
var tmpfsDefaults = []string{".idea", ".vscode"}

// allowDefaults are the egress domains DefaultsToken expands to: the wall's built-in allow.
var allowDefaults = []string{
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

	// Anthropic / Claude Code
	"platform.claude.com",
	"api.anthropic.com",
	"mcp-proxy.anthropic.com",
	"statsig.anthropic.com",
	"sentry.io",
}

// Config is the parsed .ccbox.yaml.
type Config struct {
	Tmpfs     []string          `yaml:"tmpfs"`     // workspace-relative dirs to mask with a writable tmpfs
	Env       map[string]string `yaml:"env"`       // extra env vars set in the container
	Allowlist []string          `yaml:"allowlist"` // egress wall domains; "ccbox-defaults" expands to the built-ins
}

// Defaulted resolves c against its workspace. Applied once by Load.
func Defaulted(workspaceDir string, c Config) Config {
	c.Tmpfs = append(presentTmpfsDefaults(workspaceDir), c.Tmpfs...)
	c.Allowlist = expandAllowlist(c.Allowlist)
	return c
}

// presentTmpfsDefaults returns the tmpfsDefaults that exist as dirs under workspaceDir.
func presentTmpfsDefaults(workspaceDir string) []string {
	var out []string
	for _, d := range tmpfsDefaults {
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

// Load reads workspaceDir/.ccbox.yaml and applies Defaulted. A missing file is not an
// error — it yields the defaulted zero Config, so repos without one keep working.
func Load(workspaceDir string) (Config, error) {
	body, err := os.ReadFile(filepath.Join(workspaceDir, fileName))
	if errors.Is(err, fs.ErrNotExist) {
		return Defaulted(workspaceDir, Config{}), nil
	}
	if err != nil {
		return Config{}, err
	}

	var c Config
	if err := yaml.Unmarshal(body, &c); err != nil {
		return Config{}, err
	}
	return Defaulted(workspaceDir, c), nil
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
