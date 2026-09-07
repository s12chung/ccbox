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
	// userFileName is the user-level config's name under ConfigDir (no dot)
	userFileName = "ccbox.yaml"
	// projectFileName is the project-level config's name at the project repo root
	projectFileName = ".ccbox.yaml"
	// localFileName is the project-local, git-ignored override
	localFileName = ".ccbox.local.yaml"
	// DefaultsToken listed in allowlist, expands in place to allowDefaults
	DefaultsToken = "ccbox-defaults" // #nosec G101 -- config expansion keyword, not a credential
)

// UserConfigFile is the user-level config's path
func UserConfigFile() string { return filepath.Join(userdir.ConfigDir(), userFileName) }

// layerPaths lists the config file paths in load order: user, project, local
func layerPaths(projectDir string) []string {
	return []string{UserConfigFile(), filepath.Join(projectDir, projectFileName), filepath.Join(projectDir, localFileName)}
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
	path := filepath.Join(projectDir, projectFileName)
	body, err := Config{}.renderTmpl()
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
		TmpfsMasks:    []string{DefaultsToken},
		VolumeMasks:   []string{DefaultsToken},
		Allowlist:     []string{DefaultsToken},
	}
}

// SeedUserConfig safe-seeds the user-level config with ccbox's own defaults, naming the
// seeded cli. An existing file is never touched. Returns the path seeded, or "" when it
// already exists.
func SeedUserConfig(cli string) (string, error) {
	if !ioutil.Missing(UserConfigFile()) {
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

// LoadExpanded reads the config layers — user (ConfigDir), project (project repo root),
// local (git-ignored) — then CLI flags, and validates. The DefaultsToken in each list
// field expands to the full built-ins.
func LoadExpanded(projectDir string, flags Config) (Config, error) {
	var c Config
	for _, path := range layerPaths(projectDir) {
		layer, err := read(path)
		if err != nil {
			return Config{}, err
		}
		c = c.merge(layer)
	}
	c = c.merge(flags)
	if errMap := firm.ValidateAny(c); errMap != nil {
		return Config{}, errMap
	}
	c.TmpfsMasks = expandList(c.TmpfsMasks, tmpfsDefaults)
	c.VolumeMasks = expandList(c.VolumeMasks, volumeDefaults)
	c.Allowlist = expandList(c.Allowlist, allowDefaults())
	return c, nil
}

// Load mask-checks LoadExpanded: the tmpfsMasks and volumeMasks lists keep only the project's
// present paths, so run never creates a masked path.
func Load(projectDir string, flags Config) (Config, error) {
	c, err := LoadExpanded(projectDir, flags)
	if err != nil {
		return Config{}, err
	}
	c.TmpfsMasks = ioutil.PathsPresent(projectDir, c.TmpfsMasks)
	c.VolumeMasks = ioutil.PathsPresent(projectDir, c.VolumeMasks)
	return c, nil
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
