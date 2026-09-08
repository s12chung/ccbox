package cmd

import (
	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/util/log"
)

var pkgerCmd = &cobra.Command{
	Use:   "pkger",
	Short: "Print the CLI_PKGER env JSON for the effective config",
	RunE: func(_ *cobra.Command, _ []string) error {
		body, err := harness.MustFor(*projectCfg.CLI).PkgerJSON()
		if err != nil {
			return err
		}
		log.Info(body)
		return nil
	},
}
