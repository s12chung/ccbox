package projectcfg

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"

	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/kit/yamlutil"
	"github.com/s12chung/ccbox/pkg/util/mergeempty"
)

// Config is the parsed .ccbox.yaml.
type Config struct {
	CLI           *string `yaml:"cli"`             // coding CLI to install + launch
	HostGitConfig *bool   `yaml:"host_git_config"` // read-only mount host ~/.config/git
	// camelCase keys: they read as the masks for tmpfs/volumes
	TmpfsMasks  []string          `yaml:"tmpfsMasks"`  //nolint:tagliatelle // project-relative dirs to mask with a writable tmpfs
	VolumeMasks []string          `yaml:"volumeMasks"` //nolint:tagliatelle // project-relative dirs to mask with a persistent per-project volume
	Env         map[string]string `yaml:"env"`         // extra env vars set in the container
	Allowlist   []string          `yaml:"allowlist"`   // egress wall domains
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[Config]().
		NotNil("CLI", "HostGitConfig").
		Validates(firm.RuleMap{
			"CLI": {rule.OneOf[string]{Values: harness.Names()}},

			// mask dirs are project-relative: no absolute paths, no ".." traversal
			"TmpfsMasks":  {firm.Elems[[]string](firmrule.MaskDir)},
			"VolumeMasks": {firm.Elems[[]string](firmrule.MaskDir)},
			"Env": {
				firm.Keys[map[string]string](firmrule.EnvVar),
				firm.Values[map[string]string](rule.Present{}),
			},
			"Allowlist": {firm.Elems[[]string](firmrule.Domain)},
		}))
}

// VolumeCleanupDirs is every mask dir whose volume may exist
func (c Config) VolumeCleanupDirs() []string {
	seen := map[string]bool{}
	var dirs []string
	for _, d := range append(append([]string{}, volumeDefaults...), c.VolumeMasks...) {
		if seen[d] {
			continue
		}
		seen[d] = true
		dirs = append(dirs, d)
	}
	return dirs
}

// merge layers other onto c and returns a fresh Config
func (c Config) merge(other Config) Config {
	c.TmpfsMasks = mergeempty.Slice(c.TmpfsMasks, other.TmpfsMasks)
	c.VolumeMasks = mergeempty.Slice(c.VolumeMasks, other.VolumeMasks)
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

var (
	// configTmplSrc is the ccbox.yaml template
	//
	//go:embed ccbox.yaml.tmpl
	configTmplSrc string
	configTmpl    = template.Must(template.New("ccbox.yaml").Funcs(template.FuncMap{
		"yaml": yamlutil.Value,
	}).Parse(configTmplSrc))
)

func (c Config) renderTmpl() (string, error) {
	var b bytes.Buffer
	if err := configTmpl.Execute(&b, c); err != nil {
		return "", fmt.Errorf("render Config template: %w", err)
	}
	return b.String(), nil
}
