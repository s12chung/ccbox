package docker

import (
	"context"
	"io/fs"
	"os"

	"github.com/docker/buildx/build"
	"github.com/docker/buildx/builder"
	_ "github.com/docker/buildx/driver/docker" // registers the daemon's embedded BuildKit driver factory
	"github.com/docker/buildx/util/confutil"
	"github.com/docker/buildx/util/dockerutil"
	"github.com/docker/buildx/util/progress"
	"github.com/docker/cli/cli/command"
	cliflags "github.com/docker/cli/cli/flags"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/util/progress/progressui"

	"github.com/s12chung/ccbox/pkg/util/tarutil"
)

// BuildOptions configures an image build.
type BuildOptions struct {
	Tag       string
	BuildArgs map[string]string
}

// Build builds the devbox image from the embedded build context (src) on BuildKit,
// in-process via the buildx library, streaming progress to stderr and loading the
// result into the local daemon's image store. src must hold the Dockerfile and every
// path it COPYs.
func Build(ctx context.Context, src fs.FS, o BuildOptions) error {
	contextTar, err := tarutil.ToTar(src, nil)
	if err != nil {
		return err
	}

	// buildx hangs all its machinery off a docker CLI instance.
	dockerCli, err := command.NewDockerCli()
	if err != nil {
		return err
	}
	if err := dockerCli.Initialize(cliflags.NewClientOptions()); err != nil {
		return err
	}

	// The default builder is the daemon's embedded BuildKit; resolve it to its nodes.
	b, err := builder.New(dockerCli)
	if err != nil {
		return err
	}
	nodes, err := b.LoadNodes(ctx)
	if err != nil {
		return err
	}

	printer, err := progress.NewPrinter(ctx, os.Stderr, progressui.AutoMode)
	if err != nil {
		return err
	}

	opts := map[string]build.Options{"default": {
		// Context tar on stdin (ContextPath "-"); the Dockerfile rides at its root.
		Inputs:    build.Inputs{ContextPath: "-", InStream: build.NewSyncMultiReader(contextTar)},
		Tags:      []string{o.Tag},
		BuildArgs: o.BuildArgs,
		// ExporterDocker loads the built image into the daemon store (the --load equivalent).
		// Attrs must be non-nil: buildx writes the tag into it ("name") without nil-checking.
		Exports: []client.ExportEntry{{Type: client.ExporterDocker, Attrs: map[string]string{}}},
	}}

	_, buildErr := build.Build(ctx, nodes, opts, dockerutil.NewClient(dockerCli), confutil.NewConfig(dockerCli), printer)
	// Wait drains the progress stream; surface its error only if the build itself passed.
	if waitErr := printer.Wait(); buildErr == nil {
		buildErr = waitErr
	}
	return buildErr
}
