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
	return safeSeed(seedConfigs[cli.Name], cli.SeedSrcFolder, filepath.Join(cacheDir, cli.SeedSrcFolder), cli.SeedRenames, confirm)
}

// seedTreeFn is seed.Tree, indirected so tests can stub out the file-copying step.
var seedTreeFn = seed.Tree

// safeSeed seeds dst from root's embedded tree at srcSub under seed.SourceDir when dst
// doesn't exist yet, applying renames — or re-seeds after confirmation when confirm is
// set, backing up overwritten files.
func safeSeed(root fs.FS, srcSub, dst string, renames map[string]string, confirm bool) (string, error) {
	switch _, err := os.Stat(dst); {
	case err == nil: // exists
		if !confirm {
			return dst, nil
		}
		if !prompt.Confirm(dst + " exists; reseed and back up overwritten files?") {
			log.Info("reseed aborted")
			return dst, nil
		}
	case !os.IsNotExist(err): // stat failed for some other reason
		return dst, err
	}

	src, err := fs.Sub(root, path.Join(seed.SourceDir, srcSub))
	if err != nil {
		return dst, err
	}
	renamed, err := seedTreeFn(src, dst, renames)
	if err != nil {
		return dst, err
	}
	if len(renamed) == 0 {
		log.Infof("seeded: %s", dst)
	} else {
		rel := make([]string, len(renamed))
		for i, p := range renamed {
			rel[i], _ = filepath.Rel(dst, p)
		}
		log.Infof("seeded %s, backed up overwritten files:\n%s", dst, strings.Join(rel, "\n"))
	}
	return dst, nil
}
