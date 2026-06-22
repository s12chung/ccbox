package cmd

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/prompt"
	"github.com/s12chung/ccbox/pkg/seed"
)

const claudeConfigPrefix = "docker/seed/claude-config"

// seedClaudeFn is seed.SeedClaudeConfig, indirected so tests can stub out the file-copying step.
var seedClaudeFn = seed.SeedClaudeConfig

var reseedCmd = &cobra.Command{
	Use:   "reseed",
	Short: "Seed the host Claude config dir from the embedded seed, backing up overwrites",
	RunE: func(_ *cobra.Command, _ []string) error {
		_, err := safeSeedClaudeConfig(flagCacheDir, true)
		return err
	},
}

// safeSeedClaudeConfig seeds the host claude-config dir in cacheDir and returns that dir
func safeSeedClaudeConfig(cacheDir string, confirm bool) (string, error) {
	configDir := filepath.Join(cacheDir, "claude-config")

	switch _, err := os.Stat(configDir); {
	case err == nil: // exists
		if !confirm {
			return configDir, nil
		}
		if !prompt.Confirm(configDir + " exists; reseed and back up overwritten files?") {
			slog.Info("reseed aborted")
			return configDir, nil
		}
	case !os.IsNotExist(err): // stat failed for some other reason
		return configDir, err
	}

	src, err := fs.Sub(seedClaudeConfig, claudeConfigPrefix)
	if err != nil {
		return configDir, err
	}
	renamed, err := seedClaudeFn(src, configDir)
	if err != nil {
		return configDir, err
	}
	if len(renamed) == 0 {
		slog.Info("seeded fresh, nothing backed up", "dir", configDir)
	} else {
		slog.Info("seeded, backed up overwritten files", "dir", configDir, "files", renamed)
	}
	return configDir, nil
}
