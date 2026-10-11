// Package prov loads the provisioning spec layers — what `ccbox build` bakes
// into the image: user (userdir.Dir), project (repo root), lowest precedence first.
package prov

import (
	"bytes"
	_ "embed"
	"fmt"
	"path/filepath"
	"slices"
	"text/template"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"

	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/kit/yamlutil"
	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/mergeempty"
	"github.com/s12chung/ccbox/pkg/util/seed"
	"github.com/s12chung/ccbox/pkg/util/slicex"
)

const (
	// userConfigFileName is the user-level spec's name at userdir.Dir() (no dot)
	userConfigFileName = "ccbox-prov.yaml"
	// projectConfigFileName is the project-level spec's name at the project repo root
	projectConfigFileName = ".ccbox-prov.yaml"
)

// userConfigFile is the user-level spec's path
func userConfigFile() string { return filepath.Join(userdir.Dir(), userConfigFileName) }

// projectConfigFile is dir's project-level spec path
func projectConfigFile(dir string) string { return filepath.Join(dir, projectConfigFileName) }

// Spec is the parsed provisioning spec: which runtimes the image bakes in and
// which extra apt packages it keeps. The mise config stays the source of truth
// for the other tools.
type Spec struct {
	Runtimes    []string `yaml:"runtimes"`     // selectable runtimes — node is installed by default
	AptPackages []string `yaml:"apt_packages"` // extra apt packages
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[Spec]().
		Validates(firm.RuleMap{
			"Runtimes":    {firm.Elems[[]string](rule.ShowValue(rule.OneOf[string]{ValuesFunc: selectableRuntimes}))},
			"AptPackages": {firm.Elems[[]string](rule.ShowValue(firmrule.AptPkg))},
		}))
}

func selectableRuntimes() []string {
	return slicex.Minus(runtime.Names(), []string{runtime.Node().Name})
}

// merge overlays other onto s: lists append, like cfg's layers
func (s *Spec) merge(other Spec) {
	mergeempty.Merge(s, other)
}

// Resolve resolves the spec: the resolved runtimes plus node, which is
// installed by default, and the extra apt packages. Unknown names can't
// survive Load's validation — they're skipped here, so Resolve is infallible.
func (s *Spec) Resolve() Resolved {
	names := append([]string{runtime.Node().Name}, s.Runtimes...) // node rides every resolution
	resolved := make([]runtime.Runtime, 0, len(names))
	for _, rt := range runtime.All() {
		if slices.Contains(names, rt.Name) {
			resolved = append(resolved, rt)
		}
	}
	return Resolved{Runtimes: resolved, AptPackages: slicex.Unique(s.AptPackages)}
}

// Load reads the spec layers in load order — user, project — and validates the merge
func Load(projectDir string) (*Spec, error) {
	layers, err := yamlutil.ReadLayers[Spec]([]string{userConfigFile(), projectConfigFile(projectDir)})
	if err != nil {
		return nil, err
	}
	provSpec, err := yamlutil.ValidatedMerge(layers, (*Spec).merge)
	if err != nil {
		return nil, err
	}
	return &provSpec, nil
}

// Resolved is the one input the mise sync, the Dockerfile regions and the run's
// cache volumes compose from.
type Resolved struct {
	// Runtimes is the selection plus node, in All()'s order
	Runtimes []runtime.Runtime
	// AptPackages is the spec's extra apt packages, repeats dropped
	AptPackages []string
}

// MiseTools flattens the selection's tools — the mise config's [tools] input
func (r Resolved) MiseTools() []mise.Tool {
	var tools []mise.Tool
	for _, rt := range r.Runtimes {
		tools = append(tools, rt.MiseTool)
		tools = append(tools, rt.MiseToolsOptional...)
	}
	return tools
}

// CacheVolumes concatenates the selection's cache volumes
func (r Resolved) CacheVolumes() []string {
	var volumes []string
	for _, rt := range r.Runtimes {
		volumes = append(volumes, rt.CacheVolumes...)
	}
	return volumes
}

var (
	//go:embed ccbox-prov.yaml.tmpl
	specTmplSrc string
	specTmpl    = template.Must(template.New("ccbox-prov.yaml").Funcs(template.FuncMap{
		"yaml": yamlutil.Value,
	}).Parse(specTmplSrc))
)

// SeedConfig safe-seeds the user-level spec with ccbox's defaults
func SeedConfig() error {
	spec := Spec{Runtimes: selectableRuntimes(), AptPackages: []string{}}
	var b bytes.Buffer
	if err := specTmpl.Execute(&b, &spec); err != nil {
		return fmt.Errorf("render Spec template: %w", err)
	}
	return seed.SafeFile(userConfigFile(), b.Bytes())
}
