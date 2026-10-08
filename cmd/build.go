package cmd

import (
	"context"
	"io/fs"

	"github.com/spf13/cobra"

	"github.com/s12chung/ccbox/pkg/dmap"
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/mise"
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

// miseImage is the pinned mise image the Dockerfile builds from — the lock container
// runs the same mise.
var miseImage string

// initMiseImage parses miseImage from the injected embed. Called
// from Execute: a Go init() runs before main injects the embed.
func initMiseImage() {
	dockerfile, err := fs.ReadFile(embedBuildContext, "Dockerfile")
	must.Do(err)
	miseImage = must.Get(mise.ImageFromDockerfile(dockerfile))
}

// build builds the image variant the mode asks for; `run` calls it too
func build(ctx context.Context, serveVNC bool) error {
	projectDir := projectConfig.ProjectDir()
	for _, path := range []string{mise.ProjectConfigPath(projectDir), mise.UserConfigPath()} {
		if err := mise.GenerateLock(ctx, miseImage, path); err != nil {
			return err
		}
	}
	buildContext, err := mise.BuildFS(embedBuildContext, projectDir)
	if err != nil {
		return err
	}

	buildMap := dmap.NewBuildMap(flagTag, projectDir, tagByProject, serveVNC)
	return docker.Build(ctx, buildContext, buildMap.Options())
}
