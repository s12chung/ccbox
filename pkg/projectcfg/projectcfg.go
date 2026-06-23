// Package projectcfg loads the per-project .ccbox.yaml from a workspace repo root
package projectcfg

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const fileName = ".ccbox.yaml"

// Config is the parsed .ccbox.yaml.
type Config struct {
	Tmpfs []string          `yaml:"tmpfs"` // workspace-relative dirs to mask with a writable tmpfs
	Env   map[string]string `yaml:"env"`   // extra env vars set in the container
}

// Load reads workspaceDir/.ccbox.yaml. A missing file is not an error — it yields
// the zero Config, so repos without one keep working unchanged.
func Load(workspaceDir string) (Config, error) {
	body, err := os.ReadFile(filepath.Join(workspaceDir, fileName))
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}

	var c Config
	return c, yaml.Unmarshal(body, &c)
}
