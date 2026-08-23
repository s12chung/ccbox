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
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/mergeempty"
)

const fileName = ".ccbox.yaml"

const (
	// localFileName is for git-ignored override merged onto fileName: lists append, env overlays.
	localFileName = ".ccbox.local.yaml"
	// DefaultsToken listed in allowlist, expands in place to allowDefaults
	DefaultsToken = "ccbox-defaults" // #nosec G101 -- config expansion keyword, not a credential
	// defaultCliName is the coding CLI when .ccbox.yaml doesn't say.
	defaultCliName = "claude"
)

var (
	// tmpfsDefaults always-masked dirs, prepended only when present in the workspace (to prevent host creation)
	tmpfsDefaults = []string{".idea", ".vscode"}
	// volumeDefaults for persistent volume masked dirs only when present in the workspace (to prevent host creation)
	volumeDefaults = []string{"node_modules", ".venv", "vendor/bundle"}
)

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
	return path, os.WriteFile(path, []byte(defaultYAML), ioutil.File)
}

// Load reads workspaceDir/.ccbox.yaml, layers .ccbox.local.yaml onto it, then
// CLI flags. Both files are optional — absent ones contribute the zero
// Config, so repos without either keep working.
func Load(workspaceDir string, flags Config) (Config, error) {
	base, err := read(filepath.Join(workspaceDir, fileName))
	if err != nil {
		return Config{}, err
	}
	local, err := read(filepath.Join(workspaceDir, localFileName))
	if err != nil {
		return Config{}, err
	}
	c := Defaulted(workspaceDir, base.merge(local).merge(flags))
	if err := c.validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Config is the parsed .ccbox.yaml.
type Config struct {
	CLI           string            `yaml:"cli"`             // coding CLI to install + launch (default claude)
	HostGitConfig *bool             `yaml:"host_git_config"` // read-only mount host ~/.config/git; nil = default on, resolved by Defaulted
	Tmpfs         []string          `yaml:"tmpfs"`           // workspace-relative dirs to mask with a writable tmpfs
	Volumes       []string          `yaml:"volumes"`         // workspace-relative dirs to mask with a persistent per-project volume
	Env           map[string]string `yaml:"env"`             // extra env vars set in the container
	Allowlist     []string          `yaml:"allowlist"`       // egress wall domains; "ccbox-defaults" expands to the built-ins
}

// Defaulted resolves c against its workspace. Applied once by Load.
func Defaulted(workspaceDir string, c Config) Config {
	c.Tmpfs = append(ioutil.DirsPresentInSrc(workspaceDir, tmpfsDefaults), c.Tmpfs...)
	c.Volumes = append(ioutil.DirsPresentInSrc(workspaceDir, volumeDefaults), c.Volumes...)
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
		all := harness.All()
		names := make([]string, 0, len(all))
		for _, cli := range all {
			names = append(names, cli.Name)
		}
		return fmt.Errorf("cli: unknown value %q (want %q)", c.CLI, strings.Join(names, ", "))
	}
	return nil
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
	body, err := os.ReadFile(path) // #nosec G304 -- path is the workspace's own .ccbox.yaml
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
