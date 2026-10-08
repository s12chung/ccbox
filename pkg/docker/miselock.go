package docker

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/docker/docker/api/types/container"

	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

const (
	// misePlatforms locks the devbox targets only; unlocked, mise locks every platform it knows
	misePlatforms = "linux-x64,linux-arm64"
)

// MiseLockOptions configures the mise lock container run
type MiseLockOptions struct {
	Image      string // the pure-mise image
	ConfigPath string // the mise config's host path

	UID    int    // the container user, so the lock lands user-owned, not root:root
	OutDir string // the host dir taking the written lockfile
}

// GenerateMiseLock runs mise lock in a container to generate a mise lock file, returning
// its body.
func GenerateMiseLock(ctxD *dock.CtxD, o MiseLockOptions) ([]byte, error) {
	if err := dock.EnsureImageExists(ctxD, o.Image); err != nil {
		return nil, err
	}

	miseWorkMount := "/work"
	resp, err := ctxD.D.ContainerCreate(ctxD.Ctx,
		&container.Config{
			Image:      o.Image,
			User:       strconv.Itoa(o.UID),
			Entrypoint: []string{"/usr/local/bin/mise"}, // Entrypoint and Cmd are pinned: the mise image's defaults are neither
			Cmd:        []string{"lock", "--platform", misePlatforms},
			WorkingDir: miseWorkMount,
			Env:        []string{"HOME=/tmp"}, // the host uid has no home in the image — /tmp holds the lock run's caches
		},
		&container.HostConfig{
			Binds: mountSpecs([]Mount{
				NewBind(o.OutDir, miseWorkMount),
				NewBind(o.ConfigPath, miseWorkMount+"/mise.toml").ReadOnly(),
			}),
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

	body, err := os.ReadFile(filepath.Join(o.OutDir, "mise.lock")) // #nosec G304 -- OutDir is the run's own temp dir
	if err != nil {
		return nil, fmt.Errorf("mise lock wrote no lockfile: %w\n%s", err, logs.String())
	}
	return body, nil
}
