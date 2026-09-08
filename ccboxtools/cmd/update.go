package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/ccboxtools/pkg/install"
	"github.com/s12chung/ccbox/ccboxtools/pkg/pkger"
	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
)

const (
	// clisDirEnv overrides the clis root; the image mounts the global volume at the default.
	clisDirEnv = "CCBOX_CLIS_DIR"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Install the CLI's latest version into the clis volume, pruning stale versions",
	RunE: func(_ *cobra.Command, _ []string) error {
		body := os.Getenv(pkginfo.EnvVar)
		if body == "" {
			return fmt.Errorf("%s is not set", pkginfo.EnvVar)
		}
		info, err := pkginfo.FromJSON(body)
		if err != nil {
			return err
		}

		root := os.Getenv(clisDirEnv)
		if root == "" {
			root = install.DefaultRoot
		}
		return install.Run(pkger.PkgDir{Pkger: pkger.For(info), Root: root})
	},
}
