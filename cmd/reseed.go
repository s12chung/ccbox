package cmd

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/log"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/prompt"
	"github.com/s12chung/ccbox/pkg/seed"
)

const (
	claudeSeedSrcPrefix = "docker/seed/claude-config"
	codexSeedSrcPrefix  = "docker/seed/codex-config"
)

// seed{Claude,Codex}Fn are the seed.Seed* funcs, indirected so tests can stub the file-copying step.
var (
	seedClaudeFn = seed.SeedClaudeConfig
	seedCodexFn  = seed.SeedCodexConfig
)

var reseedCmd = &cobra.Command{
	Use:   "reseed",
	Short: "Seed the host config dir for the configured CLI from the embedded seed, backing up overwrites",
	RunE: func(_ *cobra.Command, _ []string) error {
		_, err := safeSeedConfig(flagCacheDir, projectCfg.CLI, true)
		return err
	},
}

// cliSeed bundles the per-CLI knobs for seeding that CLI's config dir.
type cliSeed struct {
	src       embed.FS
	srcPrefix string // the host config dir reuses its leaf (e.g. .../codex-config -> codex-config)
	seedFn    func(fs.FS, string) ([]string, error)
}

// cliSeedSpec selects the seed knobs for cli. Reads the embed vars at call time (set by Execute).
func cliSeedSpec(cli projectcfg.CLI) cliSeed {
	if cli == projectcfg.CLICodex {
		return cliSeed{seedCodexConfig, codexSeedSrcPrefix, seedCodexFn}
	}
	return cliSeed{seedClaudeConfig, claudeSeedSrcPrefix, seedClaudeFn}
}

// safeSeedConfig seeds cli's host config dir in cacheDir and returns that dir
func safeSeedConfig(cacheDir string, cli projectcfg.CLI, confirm bool) (string, error) {
	spec := cliSeedSpec(cli)
	configDir := filepath.Join(cacheDir, filepath.Base(spec.srcPrefix))

	switch _, err := os.Stat(configDir); {
	case err == nil: // exists
		if !confirm {
			return configDir, nil
		}
		if !prompt.Confirm(configDir + " exists; reseed and back up overwritten files?") {
			log.Info("reseed aborted")
			return configDir, nil
		}
	case !os.IsNotExist(err): // stat failed for some other reason
		return configDir, err
	}

	src, err := fs.Sub(spec.src, spec.srcPrefix)
	if err != nil {
		return configDir, err
	}
	renamed, err := spec.seedFn(src, configDir)
	if err != nil {
		return configDir, err
	}
	if len(renamed) == 0 {
		log.Infof("seeded, nothing to back up: %s", configDir)
	} else {
		rel := make([]string, len(renamed))
		for i, p := range renamed {
			rel[i], _ = filepath.Rel(configDir, p)
		}
		log.Infof("seeded %s, backed up overwritten files:\n%s", configDir, strings.Join(rel, "\n"))
	}
	return configDir, nil
}
