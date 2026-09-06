// Package projectcfg loads the config file layers — user (ConfigDir), project
// (workspace repo root), local (git-ignored) — lowest precedence first
package projectcfg

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"text/template"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"
	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/kit/yamlutil"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/mergeempty"
	"github.com/s12chung/ccbox/pkg/util/seed"
)

const (
	// userFileName is the user-level config's name under ConfigDir (no dot)
	userFileName = "ccbox.yaml"
	// projectFileName is the project-level config's name at the workspace repo root
	projectFileName = ".ccbox.yaml"
	// localFileName is the project-local, git-ignored override
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

// configTmplSrc is the ccbox.yaml template
//
//go:embed ccbox.yaml.tmpl
var configTmplSrc string

// configTmpl parses configTmplSrc once with the yaml value renderer.
var configTmpl = template.Must(template.New("ccbox.yaml").Funcs(template.FuncMap{
	"yaml": yamlutil.Value,
}).Parse(configTmplSrc))

// renderConfig renders the config template over c.
func renderConfig(c Config) (string, error) {
	var b bytes.Buffer
	if err := configTmpl.Execute(&b, c); err != nil {
		return "", fmt.Errorf("projectcfg: render config template: %w", err)
	}
	return b.String(), nil
}

// Init writes to workspaceDir/.ccbox.yaml and returns its path.
func Init(workspaceDir string) (string, error) {
	path := filepath.Join(workspaceDir, projectFileName)
	body, err := renderConfig(Config{})
	if err != nil {
		return "", err
	}
	if err := seed.File(path, body); err != nil {
		return "", err
	}
	return path, nil
}

// userSeedConfig is the user-level seed's data with ccbox defaults--referenced in tests
func userSeedConfig() Config {
	return Config{
		CLI:           new(defaultCliName),
		HostGitConfig: new(true),
		Allowlist:     []string{DefaultsToken},
	}
}

// SeedUserConfig safe-seeds the user-level config with ccbox's own defaults. An existing file is never touched.
// Returns the path seeded, or "" when it already exists.
func SeedUserConfig() (string, error) {
	body, err := renderConfig(userSeedConfig())
	if err != nil {
		return "", err
	}
	if err := seed.File(userConfigFile, body); err != nil {
		if errors.Is(err, seed.ErrExists) { // the no-op path: no seed, no log
			return "", nil
		}
		return "", err
	}
	return userConfigFile, nil
}

// userConfigFile is the user-level config's path. Tests point it at a temp tree.
var userConfigFile = filepath.Join(userdir.ConfigDir(), userFileName)

// Load reads the user file, then the project's .ccbox.yaml, layers the local override
// onto it, then CLI flags. All files are optional — absent ones contribute the zero
// Config, so repos without any keep working.
func Load(workspaceDir string, flags Config) (Config, error) {
	user, err := read(userConfigFile)
	if err != nil {
		return Config{}, err
	}
	project, err := read(filepath.Join(workspaceDir, projectFileName))
	if err != nil {
		return Config{}, err
	}
	local, err := read(filepath.Join(workspaceDir, localFileName))
	if err != nil {
		return Config{}, err
	}
	c := Defaulted(workspaceDir, user.merge(project).merge(local).merge(flags))
	if errMap := firm.ValidateAny(c); errMap != nil {
		return Config{}, errMap
	}
	return c, nil
}

// Config is the parsed .ccbox.yaml.
type Config struct {
	CLI           *string           `yaml:"cli"`             // coding CLI to install + launch; nil = default claude, resolved by Defaulted
	HostGitConfig *bool             `yaml:"host_git_config"` // read-only mount host ~/.config/git; nil = default on, resolved by Defaulted
	Tmpfs         []string          `yaml:"tmpfs"`           // workspace-relative dirs to mask with a writable tmpfs
	Volumes       []string          `yaml:"volumes"`         // workspace-relative dirs to mask with a persistent per-project volume
	Env           map[string]string `yaml:"env"`             // extra env vars set in the container
	Allowlist     []string          `yaml:"allowlist"`       // egress wall domains; "ccbox-defaults" expands to the built-ins
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[Config]().Validates(firm.RuleMap{
		"CLI": {rule.OneOf[string]{Values: harness.Names()}},

		// mask dirs are workspace-relative: no absolute paths, no ".." traversal
		"Tmpfs":   {firm.Elems[[]string](firmrule.MaskPath)},
		"Volumes": {firm.Elems[[]string](firmrule.MaskPath)},
		"Env": {
			firm.Keys[map[string]string](firmrule.EnvVar),
			firm.Values[map[string]string](rule.Present{}),
		},
		"Allowlist": {firm.Elems[[]string](firmrule.Domain)},
	}))
}

// Defaulted resolves c against its workspace. Applied once by Load.
func Defaulted(workspaceDir string, c Config) Config {
	c.Tmpfs = append(ioutil.DirsPresentInSrc(workspaceDir, tmpfsDefaults), c.Tmpfs...)
	c.Volumes = append(ioutil.DirsPresentInSrc(workspaceDir, volumeDefaults), c.Volumes...)
	c.Allowlist = expandAllowlist(c.Allowlist)
	if c.CLI == nil {
		c.CLI = new(defaultCliName)
	}
	if c.HostGitConfig == nil { // resolve the default-on so the effective config prints it
		on := true
		c.HostGitConfig = &on
	}
	return c
}

// merge layers other onto c and returns a fresh Config
func (c Config) merge(other Config) Config {
	c.Tmpfs = mergeempty.Slice(c.Tmpfs, other.Tmpfs)
	c.Volumes = mergeempty.Slice(c.Volumes, other.Volumes)
	c.Allowlist = mergeempty.Slice(c.Allowlist, other.Allowlist)
	c.Env = mergeempty.Map(c.Env, other.Env)
	if other.CLI != nil {
		c.CLI = other.CLI
	}
	if other.HostGitConfig != nil {
		c.HostGitConfig = other.HostGitConfig
	}
	return c
}

// read parses one layer's config file: user, project, or local. A missing file yields
// the zero Config
func read(path string) (Config, error) {
	body, err := os.ReadFile(path) // #nosec G304 -- path is the user's or project's own ccbox.yaml
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(body))
	dec.KnownFields(true) // a typo'd key must error, not silently no-op
	switch err := dec.Decode(&c); {
	case errors.Is(err, io.EOF): // an empty file is "unset" like a missing one
		return Config{}, nil
	case err != nil:
		return Config{}, fmt.Errorf("projectcfg: parse %s: %w", path, err)
	}
	return c, nil
}
