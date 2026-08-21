package cmd

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/log"
	"github.com/s12chung/ccbox/pkg/prompt"
	"github.com/s12chung/ccbox/pkg/seed"
)

// seedTreeFn is seed.Tree, indirected so tests can stub out the file-copying step.
var seedTreeFn = seed.Tree

var reseedCmd = &cobra.Command{
	Use:   "reseed",
	Short: "Seed the host config dir for the configured CLI from the embedded seed, backing up overwrites",
	RunE: func(_ *cobra.Command, _ []string) error {
		_, err := safeSeedConfig(flagCacheDir, projectCfg.CLI, true)
		return err
	},
}

// safeSeedConfig seeds cli's host config dir in cacheDir and returns that dir
func safeSeedConfig(cacheDir string, cliName harness.Name, confirm bool) (string, error) {
	cli := harness.MustFor(cliName)
	configDir := filepath.Join(cacheDir, cli.SeedSrcFolder)

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

	src, err := fs.Sub(seedConfigs[cli.Name], path.Join(seed.SourceDir, cli.SeedSrcFolder))
	if err != nil {
		return configDir, err
	}
	renamed, err := seedTreeFn(src, configDir, cli.SeedRenames)
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
