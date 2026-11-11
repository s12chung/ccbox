// Package app holds the Setup shared by the apps ccbox installs and launches —
// CLI and GUIApp carry the same fields, rules and seed semantics.
package app

import (
	"path"
	"strings"

	"github.com/s12chung/firm"
	"github.com/s12chung/firm/rule"

	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/models/provider"
	"github.com/s12chung/ccbox/pkg/util/mergeempty"
	"github.com/s12chung/ccbox/pkg/util/slug"
)

// Setup is what ccbox sets up for an app in the container.
type Setup struct {
	// Env is the container env the app requires on every run.
	Env map[string]string `yaml:"env"`

	// DataBinds binds host per-app data into the container: key is a $HOME-relative
	// path (file or dir); value is seed content for files, nil for dirs. A file
	// nested in a bound dir is carried by that bind.
	DataBinds map[string]*string `yaml:"data_binds"`

	// Proxied is what the app needs only on proxied runs.
	Proxied Proxied `yaml:"proxied"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[Setup]().
		Validates(firm.RuleMap{
			"Env":       {firm.Keys[map[string]string](firmrule.EnvVar), firm.Values[map[string]string](rule.Present{})},
			"DataBinds": {firm.Keys[map[string]*string](firmrule.MaskDir)},
			"Proxied":   {firm.Backed()},
		}))
}

// Proxied is Setup, but only applies when there is a proxy
type Proxied struct {
	// AllowDomains are the egress wall domains this app talks to.
	AllowDomains []string `yaml:"allow_domains"`
	// DataBinds binds host per-app data like Setup.DataBinds
	DataBinds map[string]*string `yaml:"data_binds"`
}

func init() {
	firm.MustRegisterType(firm.NewDefinition[Proxied]().
		Validates(firm.RuleMap{
			"AllowDomains": {firm.Elems[[]string](firmrule.DomainOrAlias(provider.Aliases))},
			"DataBinds":    {firm.Keys[map[string]*string](firmrule.MaskDir)},
		}))
}

// binds overlays the proxied data binds onto the plain ones under the egress wall.
func (s Setup) binds(noProxy bool) map[string]*string {
	if noProxy {
		return s.DataBinds
	}
	return mergeempty.Map(s.DataBinds, s.Proxied.DataBinds)
}

// DataBindSeedMap is binds re-keyed into the seeds' host paths: the path slugged —
// nested under a bound dir, the dir's slug joined with the rest of the path as-is.
func (s Setup) DataBindSeedMap(noProxy bool) map[string]*string {
	binds := s.binds(noProxy)
	out := make(map[string]*string, len(binds))
	for containerPath, content := range binds {
		hostPath := slug.Path(containerPath)
		if ancestorPath := ancestorDir(binds, containerPath); ancestorPath != "" {
			hostPath = path.Join(slug.Path(ancestorPath), strings.TrimPrefix(containerPath, ancestorPath+"/"))
		}
		out[hostPath] = content
	}
	return out
}

// DataBindMounts pairs each bind's host path with its container bind: a seeded
// file with an ancestor dir mounted is skipped, as the ancestor dir is mounted
func (s Setup) DataBindMounts(noProxy bool) map[string]string {
	binds := s.binds(noProxy)
	out := make(map[string]string, len(binds))
	for containerPath, content := range binds {
		if ancestorPath := ancestorDir(binds, containerPath); content == nil || ancestorPath == "" {
			out[slug.Path(containerPath)] = containerPath
		}
	}
	return out
}

// DataBindContainerDirs lists the dirs to pre-create for the data binds,
// home-relative — each dir bind itself and each file bind's parent.
func (s Setup) DataBindContainerDirs(noProxy bool) []string {
	binds := s.binds(noProxy)
	dirs := make([]string, 0, len(binds))
	for containerPath, content := range binds {
		if content == nil {
			dirs = append(dirs, containerPath)
			continue
		}
		if dir := path.Dir(containerPath); dir != "." { // skip home, no binding
			dirs = append(dirs, dir)
		}
	}
	return dirs
}

// ancestorDir is containerPath's nearest ancestor that is an explicitly bound dir
func ancestorDir(binds map[string]*string, containerPath string) string {
	for dir := path.Dir(containerPath); dir != "."; dir = path.Dir(dir) {
		if content, ok := binds[dir]; ok && content == nil {
			return dir
		}
	}
	return ""
}
