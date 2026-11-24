package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"

	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/util/term"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

const (
	misePlatforms      = "linux-x64,linux-arm64" // devbox targets only
	miseWorkspaceMount = "/workspace"
)

// MiseLockOptions configures the mise lock container run
type MiseLockOptions struct {
	Image      string   // the pure-mise image
	ConfigPath string   // the mise config's host path
	Args       []string // the mise lock flags passed through, e.g. --bump
	DryRun     bool     // preview the resolution — mise writes nothing, so MiseLock comes back empty
}

// MiseLock is the lock generation's output: the lockfile's body and its dependency
// sidecars, keyed by path relative to the workspace's .mise/locks
type MiseLock struct {
	Lock     []byte
	Sidecars map[string][]byte
}

// GenerateMiseLock runs mise lock in a container to generate a mise lock file and its
// dependency sidecars.
func GenerateMiseLock(ctxD *dock.CtxD, o MiseLockOptions) (MiseLock, error) {
	if err := dock.EnsureImageExists(ctxD, o.Image); err != nil {
		return MiseLock{}, err
	}

	cmd := append([]string{"lock", "--platform", misePlatforms}, o.Args...)
	if o.DryRun {
		cmd = append(cmd, "--dry-run")
	}
	env := []string{"MISE_TRUSTED_CONFIG_PATHS=" + miseWorkspaceMount}
	if term.IsTerminal(os.Stdout) {
		env = append(env, "CLICOLOR_FORCE=1")
	}
	resp, err := ctxD.D.ContainerCreate(ctxD.Ctx,
		&container.Config{
			Image:      o.Image,
			Entrypoint: []string{"/usr/local/bin/mise"},
			Cmd:        cmd,
			WorkingDir: miseWorkspaceMount,
			Env:        env,
		},
		&container.HostConfig{
			Binds: mountSpecs([]Mount{NewBind(o.ConfigPath, miseWorkspaceMount+"/mise.toml").ReadOnly()}),
			Tmpfs: map[string]string{"/tmp": ""}, // npm backend needs this dir
		},
		nil, nil, "")
	if err != nil {
		return MiseLock{}, err
	}
	defer log.Defer("remove mise lock container", func() error {
		return ctxD.D.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
	})

	var logs bytes.Buffer
	if err := dock.RunOnce(ctxD, resp.ID, io.MultiWriter(os.Stdout, &logs)); err != nil {
		return MiseLock{}, fmt.Errorf("mise lock: %w\n%s", err, logs.String())
	}
	if o.DryRun {
		return MiseLock{}, nil
	}

	body, err := dock.CopyFileFrom(ctxD, resp.ID, miseWorkspaceMount+"/mise.lock")
	if err != nil {
		return MiseLock{}, fmt.Errorf("mise lock wrote no lockfile: %w\n%s", err, logs.String())
	}
	sidecars, err := copySidecars(ctxD, resp.ID)
	if err != nil {
		return MiseLock{}, err
	}
	return MiseLock{Lock: body, Sidecars: sidecars}, nil
}

// copySidecars copies the workspace's dependency sidecars dir out of the container.
func copySidecars(ctxD *dock.CtxD, id string) (map[string][]byte, error) {
	sidecars, err := dock.CopyDirFrom(ctxD, id, miseWorkspaceMount+"/.mise/locks")
	if errdefs.IsNotFound(err) { // no sidecars dir: no tool carries a dependency graph
		return map[string][]byte{}, nil
	}
	return sidecars, err
}
