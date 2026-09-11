package projectcfg

import (
	"bytes"
	_ "embed"
	"fmt"
	"path/filepath"
	"slices"
	"text/template"

	"github.com/gobwas/glob"
	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"

	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/kit/globkit"
	"github.com/s12chung/ccbox/pkg/kit/yamlutil"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/mergeempty"
)

// Config is the parsed .ccbox.yaml.
type Config struct {
	CLI           *string           `yaml:"cli"`             // coding CLI to install + launch
	HostGitConfig *bool             `yaml:"host_git_config"` // read-only mount host ~/.config/git
	TmpfsMasks    []string          `yaml:"tmpfs_masks"`     // project-relative dirs to mask with a writable tmpfs
	VolumeMasks   []string          `yaml:"volume_masks"`    // project-relative dirs to mask with a persistent per-project volume
	ReadOnlyGlobs []string          `yaml:"read_only_globs"` // project-relative globs to re-mount read-only
	Env           map[string]string `yaml:"env"`             // extra env vars set in the container
	Allowlist     []string          `yaml:"allowlist"`       // egress wall domains

	// projectDir anchors the masks' present-filters
	projectDir string
	// cache the DefaultsToken expansions of raw lists
	expandedTmpfsMasks    []string
	expandedVolumeMasks   []string
	expandedReadOnlyGlobs []string
	expandedAllowlist     []string
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[Config]().
		NotNil("CLI", "HostGitConfig").
		Validates(firm.RuleMap{
			"CLI":           {rule.OneOf[string]{Values: harness.Names()}},
			"HostGitConfig": {firmrule.HasValidGitDir{}},

			// mask dirs are project-relative: no absolute paths, no ".." traversal
			"TmpfsMasks":    {firm.Elems[[]string](firmrule.MaskDir)},
			"VolumeMasks":   {firm.Elems[[]string](firmrule.MaskDir)},
			"ReadOnlyGlobs": {firm.Elems[[]string](firmrule.MaskGlob)},
			"Env": {
				firm.Keys[map[string]string](firmrule.EnvVar),
				firm.Values[map[string]string](rule.Present{}),
			},
			"Allowlist": {firm.Elems[[]string](firmrule.Domain)},
		}))
}

// ProjectDir is the project dir the config was loaded from
func (c *Config) ProjectDir() string { return c.projectDir }

// merge layers other onto c — lists append, a set later scalar wins
func (c *Config) merge(other Config) {
	c.TmpfsMasks = mergeempty.Slice(c.TmpfsMasks, other.TmpfsMasks)
	c.VolumeMasks = mergeempty.Slice(c.VolumeMasks, other.VolumeMasks)
	c.ReadOnlyGlobs = mergeempty.Slice(c.ReadOnlyGlobs, other.ReadOnlyGlobs)
	c.Allowlist = mergeempty.Slice(c.Allowlist, other.Allowlist)
	c.Env = mergeempty.Map(c.Env, other.Env)
	if other.CLI != nil {
		c.CLI = other.CLI
	}
	if other.HostGitConfig != nil {
		c.HostGitConfig = other.HostGitConfig
	}
	c.expandedTmpfsMasks = nil
	c.expandedVolumeMasks = nil
	c.expandedReadOnlyGlobs = nil
	c.expandedAllowlist = nil
}

// TmpfsMasksExpanded expands the DefaultsToken tokens in TmpfsMasks to defaults
func (c *Config) TmpfsMasksExpanded() []string {
	if c.expandedTmpfsMasks == nil {
		c.expandedTmpfsMasks = expandList(c.TmpfsMasks, tmpfsDefaults)
	}
	return c.expandedTmpfsMasks
}

// TmpfsMasksPresent filters TmpfsMasksExpanded() to the paths present as dirs in the project
func (c *Config) TmpfsMasksPresent() []string {
	return ioutil.DirsPresent(c.projectDir, c.TmpfsMasksExpanded())
}

// TmpfsMasksAbsent filters TmpfsMasksExpanded() to the paths NOT present as dirs in the project
func (c *Config) TmpfsMasksAbsent() []string {
	return absentDirs(c.projectDir, c.TmpfsMasksExpanded())
}

// VolumeMasksExpanded expands the DefaultsToken tokens in VolumeMasks to defaults
func (c *Config) VolumeMasksExpanded() []string {
	if c.expandedVolumeMasks == nil {
		c.expandedVolumeMasks = expandList(c.VolumeMasks, volumeDefaults)
	}
	return c.expandedVolumeMasks
}

// VolumeMasksPresent filters VolumeMasksExpanded() to the paths present as dirs in the project
func (c *Config) VolumeMasksPresent() []string {
	return ioutil.DirsPresent(c.projectDir, c.VolumeMasksExpanded())
}

// VolumeMasksAbsent filters VolumeMasksExpanded() to the paths NOT present as dirs in the project
func (c *Config) VolumeMasksAbsent() []string {
	return absentDirs(c.projectDir, c.VolumeMasksExpanded())
}

// ReadOnlyGlobsExpanded expands the DefaultsToken tokens in ReadOnlyGlobs to defaults
func (c *Config) ReadOnlyGlobsExpanded() []string {
	if c.expandedReadOnlyGlobs == nil {
		c.expandedReadOnlyGlobs = expandList(c.ReadOnlyGlobs, readOnlyDefaults)
	}
	return c.expandedReadOnlyGlobs
}

func (c *Config) readOnlyGlobsMatchers() []glob.Glob {
	globStrings := c.ReadOnlyGlobsExpanded()
	globs := make([]glob.Glob, 0, len(globStrings))
	for _, g := range globStrings {
		globs = append(globs, globkit.DoubleStarRooted(g))
	}
	return globs
}

// ReadOnlyPathsPresent matches ReadOnlyGlobsExpanded() against the project to actual paths,
// minus masked dirs and covered matches — a kept match covers the matches under it
func (c *Config) ReadOnlyPathsPresent() []string {
	covered := map[string]bool{}
	for _, mask := range slices.Concat(c.TmpfsMasksExpanded(), c.VolumeMasksExpanded()) {
		covered[mask] = true
	}
	var paths []string
	for _, match := range globkit.WalkMatches(c.projectDir, c.readOnlyGlobsMatchers()...) {
		dir := match
		for dir != "." && !covered[dir] {
			dir = filepath.Dir(dir)
		}
		if dir != "." {
			continue // a mask or a kept match covers it
		}
		covered[match] = true
		paths = append(paths, match)
	}
	slices.Sort(paths)
	return paths
}

// AllowlistExpanded expands the DefaultsToken tokens in Allowlist to AllowDefaults
func (c *Config) AllowlistExpanded() []string {
	if c.expandedAllowlist == nil {
		c.expandedAllowlist = expandList(c.Allowlist, AllowDefaults())
	}
	return c.expandedAllowlist
}

// MarshalYAML renders the effective config: masks resolved to the project's present
// dirs, read_only_globs to their matched paths, list tokens expanded — what `ccbox config` prints.
func (c *Config) MarshalYAML() (any, error) {
	type resolved Config // same yaml tags, no MarshalYAML method
	return resolved{
		CLI:           c.CLI,
		HostGitConfig: c.HostGitConfig,
		TmpfsMasks:    c.TmpfsMasksPresent(),
		VolumeMasks:   c.VolumeMasksPresent(),
		ReadOnlyGlobs: c.ReadOnlyPathsPresent(),
		Env:           c.Env,
		Allowlist:     c.AllowlistExpanded(),
	}, nil
}

// expandList replaces each DefaultsToken with defaults, preserving entry order and
// dropping repeat entries.
func expandList(list, defaults []string) []string {
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
		} else {
			add(d)
		}
	}
	return out
}

var (
	// configTmplSrc is the ccbox.yaml template
	//
	//go:embed ccbox.yaml.tmpl
	configTmplSrc string
	configTmpl    = template.Must(template.New("ccbox.yaml").Funcs(template.FuncMap{
		"yaml": yamlutil.Value,
	}).Parse(configTmplSrc))
)

func (c *Config) renderTmpl() (string, error) {
	var b bytes.Buffer
	if err := configTmpl.Execute(&b, c); err != nil {
		return "", fmt.Errorf("render Config template: %w", err)
	}
	return b.String(), nil
}

func absentDirs(src string, dirs []string) []string {
	present := ioutil.DirsPresent(src, dirs)
	var out []string
	for _, d := range dirs {
		if !slices.Contains(present, d) {
			out = append(out, d)
		}
	}
	return out
}
