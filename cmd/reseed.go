package cmd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/cli"
	"github.com/s12chung/ccbox/pkg/cli/clitmpl"
	"github.com/s12chung/ccbox/pkg/kit/pick"
	"github.com/s12chung/ccbox/pkg/util/fsync"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

var reseedCmd = &cobra.Command{
	Use:   "reseed",
	Short: "Seed the host config dir for the configured CLI from the embedded seed, backing up overwrites",
	RunE: func(_ *cobra.Command, _ []string) error {
		return safeSeedCLIConfig(*projectConfig.CLIName, true)
	},
}

// safeSeedCLIConfig seeds cli's host config dir
func safeSeedCLIConfig(cliName string, confirm bool) error {
	// any cliName passed down will match a CLI in All() - see the cli package NOTE
	c := cli.MustFor(cliName)
	return safeSeed(clitmpl.UserCLIConfigSeedFS(cliName, c.IsUserDefined()), cli.UserConfigDir(cliName), confirm)
}

// seedFn is fsync.Seed, indirected so tests can stub out the file-copying step.
var seedFn = fsync.Seed

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

	renamed, err := seedFn(fsys, dst)
	switch {
	case errors.Is(err, fsync.ErrNoChanges):
		if confirm {
			log.Infof("no seed changes")
		}
		return nil
	case err != nil:
		return err
	case !confirm:
		return nil // the first run's seed stays quiet
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
