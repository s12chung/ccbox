package cmd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/kit/pick"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/seed"
)

var reseedCmd = &cobra.Command{
	Use:   "reseed",
	Short: "Seed the host config dir for the configured CLI from the embedded seed, backing up overwrites",
	RunE: func(_ *cobra.Command, _ []string) error {
		return safeSeedCLIConfig(userdir.Dir(), *projectCfg.CLI, true)
	},
}

// safeSeedCLIConfig seeds cli's host config dir in userDir
func safeSeedCLIConfig(userDir, cliName string, confirm bool) error {
	return safeSeed(harness.SeedCLIFS(cliName), cliConfigDir(userDir, cliName), confirm)
}

// cliConfigDir is the host dir of cli's config: userDir/<cli_name>
func cliConfigDir(userDir, cliName string) string {
	return filepath.Join(userDir, harness.MustFor(cliName).Name)
}

// seedTreeFn is seed.Tree, indirected so tests can stub out the file-copying step.
var seedTreeFn = seed.Tree

// safeSeed seeds dst from fsys when dst doesn't exist yet — or re-seeds after
// confirmation when confirm is set, backing up overwritten files.
func safeSeed(fsys fs.FS, dst string, confirm bool) error {
	switch _, err := os.Stat(dst); {
	case err == nil: // exists
		if !confirm {
			return nil
		}
		confirmed, err := pick.Confirm(dst + " exists; reseed and back up overwritten files?")
		if err != nil {
			return err
		}
		if !confirmed {
			log.Info("reseed aborted")
			return nil
		}
	case !os.IsNotExist(err): // stat failed for some other reason
		return err
	}

	renamed, err := seedTreeFn(fsys, dst)
	switch {
	case errors.Is(err, seed.ErrNoChanges):
		log.Infof("no seed changes")
	case err != nil:
		return err
	case len(renamed) == 0:
		log.Infof("seeded: %s with no overwritten files", dst)
	default:
		rel := make([]string, len(renamed))
		for i, p := range renamed {
			rel[i], _ = filepath.Rel(dst, p)
		}
		log.Infof("seeded %s, backed up overwritten files:\n%s", dst, strings.Join(rel, "\n"))
	}
	return nil
}
