package cmd

import (
	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/ccboxtools/pkg/install"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Install the CLI's latest version into the clis volume, pruning stale versions",
	RunE: func(_ *cobra.Command, _ []string) error {
		return install.RunFromEnv()
	},
}
