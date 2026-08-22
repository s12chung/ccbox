package cmd

import (
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/dockerutil"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove the egress wall network and this project's cache volumes",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		ctxD, err := dockerutil.NewCtxD(cmd.Context())
		if err != nil {
			return err
		}

		return errors.Join(
			docker.VolumeClean(ctxD, cwd, projectCfg.VolumeCleanupDirs()),
			docker.ProxyClean(ctxD),
		)
	},
}
