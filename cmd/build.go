package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/npm"
	"github.com/s12chung/ccbox/pkg/projectcfg"
)

// cliPackages maps each CLI to the npm package the image installs — the same choice the
// Dockerfile makes, mirrored here so we can resolve the selected CLI's "latest" version.
var cliPackages = map[projectcfg.CLI]string{
	projectcfg.CLIClaude: "@anthropic-ai/claude-code",
	projectcfg.CLICodex:  "@openai/codex",
}

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
	// Pin "latest" now so the image records the concrete version, not a moving tag.
	if flagCLIVersion == "latest" {
		v, err := npm.LatestVersion(cliPackages[projectCfg.CLI])
		if err != nil {
			return err
		}
		flagCLIVersion = v
	}

	return docker.Build(ctx, buildContext, docker.BuildOptions{
		Tag:        flagTag,
		CLI:        projectCfg.CLI,
		CLIVersion: flagCLIVersion,
	})
}

func init() {
	buildCmd.Flags().StringVar(&flagCLIVersion, "cli-version", "latest",
		"CLI_VERSION build arg")
}
