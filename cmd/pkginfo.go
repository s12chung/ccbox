package cmd

import (
	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
	"github.com/s12chung/ccbox/pkg/harness"
)

var pkginfoCmd = &cobra.Command{
	Use:   "pkginfo",
	Short: "Print the CLI_PKGINFO env JSON for the effective config",
	RunE: func(_ *cobra.Command, _ []string) error {
		body, err := harness.MustFor(*projectCfg.CLI).PkgInfoJSON()
		if err != nil {
			return err
		}
		log.Info(body)
		return nil
	},
}
