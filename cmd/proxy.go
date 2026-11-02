package cmd

import (
	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/dmap"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy",
	Short: "Boot the devbox with the egress wall on, streaming its logs to the foreground",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runBoot(cmd.Context(), dmap.NewProxyMap(config).Options())
	},
}
