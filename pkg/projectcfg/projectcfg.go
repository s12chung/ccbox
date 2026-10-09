// Package projectcfg loads the config file layers — user (ConfigDir), project
// (project repo root), local (git-ignored) — lowest precedence first
package projectcfg

import (
	"path/filepath"

	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/kit/yamlutil"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/errs"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/seed"
)

const (
	// userConfigFileName is the user-level config's name under ConfigDir (no dot)
	userConfigFileName = "ccbox.yaml"
	// projectConfigFileName is the project-level config's name at the project repo root
	projectConfigFileName = ".ccbox.yaml"
	// localConfigFileName is the project-local, git-ignored override
	localConfigFileName = ".ccbox.local.yaml"
	// DefaultsAlias listed in allowlist, expands in place to the shared tooling defaults
	DefaultsAlias = "ccbox-defaults"
	// SetHarnessAlias listed in allowlist, expands in place to the selected CLI's egress
	// domains — plus the GUI app's on a vnc load
	SetHarnessAlias = "ccbox-set-harness"
)

// UserConfigFile is the user-level config's path
func UserConfigFile() string { return filepath.Join(userdir.ConfigDir(), userConfigFileName) }

// layerPaths lists the config file paths in load order: user, project, local
func layerPaths(projectDir string) []string {
	return []string{UserConfigFile(), filepath.Join(projectDir, projectConfigFileName), filepath.Join(projectDir, localConfigFileName)}
}

// LoadedPaths returns the layer config paths split into loaded and not loaded, each in load order
func LoadedPaths(projectDir string) ([]string, []string) {
	var loaded, notLoaded []string
	for _, path := range layerPaths(projectDir) {
		if osutil.Present(path) {
			loaded = append(loaded, path)
		} else {
			notLoaded = append(notLoaded, path)
		}
	}
	return loaded, notLoaded
}

// Init writes a starter .ccbox.yaml to projectDir
func Init(projectDir string) error {
	return initConfig(filepath.Join(projectDir, projectConfigFileName), Config{})
}

// userSeedConfig is the user-level seed's data with ccbox defaults--referenced in tests
func userSeedConfig(cli string) *Config {
	return &Config{
		CLIName:       new(cli),
		ForwardEnv:    []string{"TERM", "COLORTERM"},
		TmpfsMasks:    []string{DefaultsAlias},
		VolumeMasks:   []string{DefaultsAlias},
		ReadOnlyGlobs: []string{DefaultsAlias},
		ReadOnlyBinds: map[string]string{GitConfigKey: firmrule.EnabledValue},
		Allowlist:     []string{DefaultsAlias, runtime.AllRuntimesAlias, SetHarnessAlias},
	}
}

// SeedConfig safe-seeds the config with ccbox's own defaults, naming the seeded cli.
func SeedConfig(path, cli string) error {
	return errs.Swallow(initConfig(path, *userSeedConfig(cli)), seed.ErrExists)
}

func initConfig(path string, config Config) error {
	body, err := config.renderTmpl()
	if err != nil {
		return err
	}
	return seed.File(path, body)
}

// Load reads the layerPaths, then overlays the CLI flags as the top layer, and validates
func Load(projectDir string, flags Config, vncFlag bool) (*Config, error) {
	layers, err := yamlutil.ReadLayers[Config](layerPaths(projectDir))
	if err != nil {
		return nil, err
	}
	c, err := yamlutil.ValidatedMerge(append(layers, flags), (*Config).merge)
	if err != nil {
		return nil, err
	}
	c.projectDir = projectDir
	c.serveVNC = vncFlag || (c.VNC != nil && c.VNC.Enabled)
	return &c, nil
}
