package cmd

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/dock"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove the egress wall network and this project's cache volumes",
	RunE: func(cmd *cobra.Command, _ []string) error {
		ctxD, err := dock.NewCtxD(cmd.Context())
		if err != nil {
			return err
		}
		return errors.Join(
			docker.VolumeClean(ctxD, projectCfg.ProjectDir()),
			docker.ProxyClean(ctxD),
		)
	},
}
