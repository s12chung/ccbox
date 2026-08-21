package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/harness"
)

var flagCLIVersion string

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the devbox image",
	RunE: func(cmd *cobra.Command, _ []string) error {
		return build(cmd.Context())
	},
}

// build builds the devbox image; `run` calls it too, mirroring the old `run: build`.
func build(ctx context.Context) error {
	cli := harness.MustFor(projectCfg.CLI)

	// Pin "latest" now so the image records the concrete version, not a moving tag.
	if flagCLIVersion == "latest" {
		v, err := cli.Pinner.Latest()
		if err != nil {
			return err
		}
		flagCLIVersion = v
	}

	return docker.Build(ctx, buildContext, docker.BuildOptions{
		Tag:        flagTag,
		CLIName:    cli.Name,
		CLIVersion: flagCLIVersion,
	})
}

func init() {
	buildCmd.Flags().StringVar(&flagCLIVersion, "cli-version", "latest",
		"CLI_VERSION build arg")
}
