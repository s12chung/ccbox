package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/dmap"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/dockerfile"
	"github.com/s12chung/ccbox/pkg/mise"
	"github.com/s12chung/ccbox/pkg/models/runtime"
	"github.com/s12chung/ccbox/pkg/util/must"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build the devbox image",
	RunE:  func(cmd *cobra.Command, _ []string) error { return build(cmd.Context(), buildVNC) },
}

// buildVNC builds the image's desktop variant (`build --vnc`)
var buildVNC bool

func init() {
	buildCmd.Flags().BoolVar(&buildVNC, "vnc", false, "build VNC variant (experimental)")
}

// build builds the image variant the mode asks for; `run` calls it too
func build(ctx context.Context, serveVNC bool) error {
	projectDir := projectConfig.ProjectDir()
	miseImageRef := must.Get(dockerfile.MiseImageRef())
	for _, path := range []string{mise.ProjectConfigPath(projectDir), mise.UserConfigPath()} {
		if err := mise.GenerateLock(ctx, miseImageRef, path); err != nil {
			return err
		}
	}
	buildContext, err := mise.BuildFS(embedBuildContext, projectDir, runtime.AllMiseTools())
	if err != nil {
		return err
	}
	buildContext, err = dockerfile.PatchFS(buildContext)
	if err != nil {
		return err
	}

	buildMap := dmap.NewBuildMap(flagTag, projectDir, tagByProject, serveVNC)
	return docker.Build(ctx, buildContext, buildMap.Options())
}
