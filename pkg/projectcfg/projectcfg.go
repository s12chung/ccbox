// Package projectcfg loads the config file layers — user (ConfigDir), project
// (project repo root), local (git-ignored) — lowest precedence first
package projectcfg

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/s12chung/firm"
	"gopkg.in/yaml.v3"

	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/pkg/util/seed"
)

const (
	// userConfigFileName is the user-level config's name under ConfigDir (no dot)
	userConfigFileName = "ccbox.yaml"
	// projectConfigFileName is the project-level config's name at the project repo root
	projectConfigFileName = ".ccbox.yaml"
	// localConfigFileName is the project-local, git-ignored override
	localConfigFileName = ".ccbox.local.yaml"
	// DefaultsToken listed in allowlist, expands in place to allowDefaults
	DefaultsToken = "ccbox-defaults" // #nosec G101 -- config expansion keyword, not a credential
)

// UserConfigFile is the user-level config's path
func UserConfigFile() string { return filepath.Join(userdir.ConfigDir(), userConfigFileName) }

// layerPaths lists the config file paths in load order: user, project, local
func layerPaths(projectDir string) []string {
	return []string{UserConfigFile(), filepath.Join(projectDir, projectConfigFileName), filepath.Join(projectDir, localConfigFileName)}
}

// LoadedPaths returns the layer config files present on disk, in load order
func LoadedPaths(projectDir string) []string {
	var present []string
	for _, path := range layerPaths(projectDir) {
		if _, err := os.Stat(path); err == nil {
			present = append(present, path)
		}
	}
	return present
}

// Init writes to projectDir/.ccbox.yaml and returns its path.
func Init(projectDir string) (string, error) {
	path := filepath.Join(projectDir, projectConfigFileName)
	body, err := new(Config).renderTmpl()
	if err != nil {
		return "", err
	}
	if err := seed.File(path, body); err != nil {
		return "", err
	}
	return path, nil
}

// userSeedConfig is the user-level seed's data with ccbox defaults--referenced in tests
func userSeedConfig(cli string) *Config {
	return &Config{
		CLIName:       new(cli),
		HostGitConfig: new(true),
		TmpfsMasks:    []string{DefaultsToken},
		VolumeMasks:   []string{DefaultsToken},
		ReadOnlyGlobs: []string{DefaultsToken},
		Allowlist:     []string{DefaultsToken},
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
	if err := seed.File(UserConfigFile(), body); err != nil {
		if errors.Is(err, seed.ErrExists) { // lost a seed race: no seed, no log
			return "", nil
		}
		return "", err
	}
	return UserConfigFile(), nil
}

// Load reads the layerPaths, then CLI flags, and validates
func Load(projectDir string, flags Config) (*Config, error) {
	var c Config
	for _, path := range layerPaths(projectDir) {
		layer, err := read(path)
		if err != nil {
			return nil, err
		}
		c.merge(layer)
	}
	c.merge(flags)
	if errMap := firm.ValidateAny(c); errMap != nil {
		return nil, errMap
	}
	c.projectDir = projectDir
	return &c, nil
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
