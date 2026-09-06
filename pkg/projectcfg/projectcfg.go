// Package projectcfg loads the config file layers — user (ConfigDir), project
// (project repo root), local (git-ignored) — lowest precedence first
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
	// projectFileName is the project-level config's name at the project repo root
	projectFileName = ".ccbox.yaml"
	// localFileName is the project-local, git-ignored override
	localFileName = ".ccbox.local.yaml"
	// DefaultsToken listed in allowlist, expands in place to allowDefaults
	DefaultsToken = "ccbox-defaults" // #nosec G101 -- config expansion keyword, not a credential
)

var (
	// tmpfsDefaults always-masked dirs, prepended only when present in the project (to prevent host creation)
	tmpfsDefaults = []string{".idea", ".vscode"}
	// volumeDefaults for persistent volume masked dirs only when present in the project (to prevent host creation)
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

// Init writes to projectDir/.ccbox.yaml and returns its path.
func Init(projectDir string) (string, error) {
	path := filepath.Join(projectDir, projectFileName)
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
func userSeedConfig(cli string) Config {
	return Config{
		CLI:           new(cli),
		HostGitConfig: new(true),
		Tmpfs:         []string{DefaultsToken},
		Volumes:       []string{DefaultsToken},
		Allowlist:     []string{DefaultsToken},
	}
}

// SeedUserConfig safe-seeds the user-level config with ccbox's own defaults, naming the
// seeded cli. An existing file is never touched. Returns the path seeded, or "" when it
// already exists.
func SeedUserConfig(cli string) (string, error) {
	if !UserConfigNeedsSeed() {
		return "", nil // the no-op path: no seed, no log
	}
	body, err := renderConfig(userSeedConfig(cli))
	if err != nil {
		return "", err
	}
	if err := seed.File(userConfigFile, body); err != nil {
		if errors.Is(err, seed.ErrExists) { // lost a seed race: no seed, no log
			return "", nil
		}
		return "", err
	}
	return userConfigFile, nil
}

// UserConfigNeedsSeed reports whether the user-level config is missing
func UserConfigNeedsSeed() bool {
	_, err := os.Stat(userConfigFile)
	return errors.Is(err, fs.ErrNotExist)
}

// UserConfigFile is the user-level config's path
func UserConfigFile() string { return userConfigFile }

// userConfigFile is the user-level config's path. Tests point it at a temp tree.
var userConfigFile = filepath.Join(userdir.ConfigDir(), userFileName)

// Load reads the user file, then the project's .ccbox.yaml, layers the local override
// onto it, then CLI flags. All files are optional — absent ones contribute the zero
// Config
func Load(projectDir string, flags Config) (Config, error) {
	user, err := read(userConfigFile)
	if err != nil {
		return Config{}, err
	}
	project, err := read(filepath.Join(projectDir, projectFileName))
	if err != nil {
		return Config{}, err
	}
	local, err := read(filepath.Join(projectDir, localFileName))
	if err != nil {
		return Config{}, err
	}
	c := ExpandDefaults(projectDir, user.merge(project).merge(local).merge(flags))
	if errMap := firm.ValidateAny(c); errMap != nil {
		return Config{}, errMap
	}
	return c, nil
}

// Config is the parsed .ccbox.yaml.
type Config struct {
	CLI           *string           `yaml:"cli"`             // coding CLI to install + launch
	HostGitConfig *bool             `yaml:"host_git_config"` // read-only mount host ~/.config/git
	Tmpfs         []string          `yaml:"tmpfs"`           // project-relative dirs to mask with a writable tmpfs
	Volumes       []string          `yaml:"volumes"`         // project-relative dirs to mask with a persistent per-project volume
	Env           map[string]string `yaml:"env"`             // extra env vars set in the container
	Allowlist     []string          `yaml:"allowlist"`       // egress wall domains
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[Config]().
		NotNil("CLI", "HostGitConfig").
		Validates(firm.RuleMap{
			"CLI": {rule.OneOf[string]{Values: harness.Names()}},

			// mask dirs are project-relative: no absolute paths, no ".." traversal
			"Tmpfs":   {firm.Elems[[]string](firmrule.MaskPath)},
			"Volumes": {firm.Elems[[]string](firmrule.MaskPath)},
			"Env": {
				firm.Keys[map[string]string](firmrule.EnvVar),
				firm.Values[map[string]string](rule.Present{}),
			},
			"Allowlist": {firm.Elems[[]string](firmrule.Domain)},
		}))
}

// ExpandDefaults expands the DefaultsToken in each list field in place
func ExpandDefaults(projectDir string, c Config) Config {
	c.Tmpfs = expandList(c.Tmpfs, ioutil.DirsPresentInSrc(projectDir, tmpfsDefaults))
	c.Volumes = expandList(c.Volumes, ioutil.DirsPresentInSrc(projectDir, volumeDefaults))
	c.Allowlist = expandList(c.Allowlist, allowDefaults())
	return c
}

// expandList replaces each DefaultsToken with defaults, preserving entry order and
// dropping repeat entries.
func expandList(list, defaults []string) []string {
	if list == nil {
		list = []string{DefaultsToken}
	}
	var out []string
	seen := map[string]bool{}
	add := func(entries ...string) {
		for _, e := range entries {
			if seen[e] {
				continue
			}
			seen[e] = true
			out = append(out, e)
		}
	}
	for _, d := range list {
		if d == DefaultsToken {
			add(defaults...)
			continue
		}
		add(d)
	}
	return out
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
