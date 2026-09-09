package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/ccboxtools/pkg/entrypoint"
	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
)

var entrypointCmd = &cobra.Command{
	Use:                   "entrypoint [--] [command...]",
	Short:                 "Verify the container's identity and egress wall, then exec the command",
	DisableFlagParsing:    true,
	DisableFlagsInUseLine: true,
	Args:                  cobra.ArbitraryArgs,
	RunE: func(_ *cobra.Command, args []string) error {
		if len(args) > 0 && args[0] == "--" { // cobra's arg split leaves the separator in
			args = args[1:]
		}
		if err := entrypoint.Run(args); err != nil {
			log.Errorf("security-entrypoint: FAIL — %v", err)
			os.Exit(1)
		}
		return nil
	},
}
