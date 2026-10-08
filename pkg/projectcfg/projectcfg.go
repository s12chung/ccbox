// Package projectcfg loads the config file layers — user (ConfigDir), project
// (project repo root), local (git-ignored) — lowest precedence first
package projectcfg

import (
	"errors"
	"path/filepath"

	"github.com/s12chung/ccbox/pkg/kit/firmrule"
	"github.com/s12chung/ccbox/pkg/kit/yamlutil"
	"github.com/s12chung/ccbox/pkg/runtime"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/fsync"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
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
		if ioutil.Present(path) {
			loaded = append(loaded, path)
		} else {
			notLoaded = append(notLoaded, path)
		}
	}
	return loaded, notLoaded
}

// Init writes to projectDir/.ccbox.yaml and returns its path.
func Init(projectDir string) (string, error) {
	path := filepath.Join(projectDir, projectConfigFileName)
	body, err := new(Config).renderTmpl()
	if err != nil {
		return "", err
	}
	if err := fsync.File(path, body); err != nil {
		return "", err
	}
	return path, nil
}

// userSeedConfig is the user-level seed's data with ccbox defaults--referenced in tests
func userSeedConfig(cli string) *Config {
	return &Config{
		CLIName:       new(cli),
		TmpfsMasks:    []string{DefaultsAlias},
		VolumeMasks:   []string{DefaultsAlias},
		ReadOnlyGlobs: []string{DefaultsAlias},
		ReadOnlyBinds: map[string]string{GitConfigKey: firmrule.EnabledValue},
		Allowlist:     []string{DefaultsAlias, runtime.AllRuntimesAlias, SetHarnessAlias},
	}
}

// SeedUserConfig safe-seeds the user-level config with ccbox's own defaults, naming the
// seeded cli. An existing file is never touched. Returns the path seeded, or "" when it
// already exists.
func SeedUserConfig(cli string) (string, error) {
	if ioutil.Present(UserConfigFile()) {
		return "", nil // the no-op path: no seed, no log
	}
	body, err := userSeedConfig(cli).renderTmpl()
	if err != nil {
		return "", err
	}
	if err := fsync.File(UserConfigFile(), body); err != nil {
		if errors.Is(err, fsync.ErrExists) { // lost a seed race: no seed, no log
			return "", nil
		}
		return "", err
	}
	return UserConfigFile(), nil
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
