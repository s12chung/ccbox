package cmd

import (
	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/entrypoint"
)

var entrypointCmd = &cobra.Command{
	Use:   "entrypoint",
	Short: "Boot the container: build and prove the egress wall, then idle as its root supervisor",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return entrypoint.Run(cmd.Context(), vnc)
	},
}
