package cmd

import (
	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/dock"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove this project's docker resources",
}

var cleanVolumesCmd = &cobra.Command{
	Use:   "volumes",
	Short: "Remove this project's cache volumes",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return docker.VolumeClean(dock.MustNewCtxD(cmd.Context()), config.ProjectDir())
	},
}

var cleanContainersCmd = &cobra.Command{
	Use:   "containers",
	Short: "Remove this project's devbox containers",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return docker.ContainerClean(dock.MustNewCtxD(cmd.Context()), config.ProjectDir())
	},
}

func init() {
	cleanCmd.AddCommand(cleanVolumesCmd, cleanContainersCmd)
}
