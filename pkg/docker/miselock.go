package docker

import (
	"bytes"
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"

	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

const (
	misePlatforms      = "linux-x64,linux-arm64" // devbox targets only
	miseWorkspaceMount = "/workspace"
)

// MiseLockOptions configures the mise lock container run
type MiseLockOptions struct {
	Image      string // the pure-mise image
	ConfigPath string // the mise config's host path
}

// GenerateMiseLock runs mise lock in a container to generate a mise lock file, returning
// its body.
func GenerateMiseLock(ctxD *dock.CtxD, o MiseLockOptions) ([]byte, error) {
	if err := dock.EnsureImageExists(ctxD, o.Image); err != nil {
		return nil, err
	}

	resp, err := ctxD.D.ContainerCreate(ctxD.Ctx,
		&container.Config{
			Image:      o.Image,
			Entrypoint: []string{"/usr/local/bin/mise"},
			Cmd:        []string{"lock", "--platform", misePlatforms},
			WorkingDir: miseWorkspaceMount,
			Env:        []string{"MISE_TRUSTED_CONFIG_PATHS=" + miseWorkspaceMount},
		},
		&container.HostConfig{
			Binds: mountSpecs([]Mount{NewBind(o.ConfigPath, miseWorkspaceMount+"/mise.toml").ReadOnly()}),
			Tmpfs: map[string]string{"/tmp": ""}, // npm backend needs this dir
		},
		nil, nil, "")
	if err != nil {
		return nil, err
	}
	defer log.Defer("remove mise lock container", func() error {
		return ctxD.D.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
	})

	var logs bytes.Buffer
	if err := dock.RunOnce(ctxD, resp.ID, &logs); err != nil {
		return nil, fmt.Errorf("mise lock: %w\n%s", err, logs.String())
	}

	body, err := dock.CopyFileFrom(ctxD, resp.ID, miseWorkspaceMount+"/mise.lock")
	if err != nil {
		return nil, fmt.Errorf("mise lock wrote no lockfile: %w\n%s", err, logs.String())
	}
	return body, nil
}
