package cmd

import (
	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/dock"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove this project's cache volumes",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctxD := dock.MustNewCtxD(cmd.Context())
		return docker.VolumeClean(ctxD, config.ProjectDir())
	},
}
