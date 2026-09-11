package docker

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

const proxyPort = "8888"

// RunOptions configures the interactive devbox container.
type RunOptions struct {
	RunHostOptions

	Tag string
	Env map[string]string // container env minus the wall's proxy vars
	Cmd []string          // command the entrypoint execs; nil uses the image default (shell)

	Proxy        ProxyOptions // configs + generated allow.txt for an auto-started wall
	ProxyLogPath string       // file an auto-started wall's logs are appended to
	NoProxy      bool         // run on plain bridge networking, no wall or proxy env
}

// RunHostOptions is everything mounted into the devbox: the workspace, the host dirs
// and volumes as mounts, and the project dirs masked with tmpfs. Cwd is the project
// identity labeling its volumes; WorkspaceMountPath is the container's WorkingDir.
type RunHostOptions struct {
	Cwd                string
	WorkspaceMountPath string
	Mounts             []Mount
	TmpfsPaths         []string
}

func runConfig(hostOptions RunOptions) *container.Config {
	return &container.Config{
		Image:        hostOptions.Tag,
		Cmd:          hostOptions.Cmd,
		WorkingDir:   hostOptions.WorkspaceMountPath,
		Tty:          true,
		OpenStdin:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Env:          envString(hostOptions.Env, hostOptions.NoProxy),
	}
}

func runHostConfig(ctxD *dock.CtxD, hostOptions RunOptions) (*container.HostConfig, error) {
	if err := ensureMounts(ctxD, hostOptions.Tag, hostOptions.Cwd, hostOptions.Mounts); err != nil {
		return nil, err
	}

	// The egress wall network by default; --no-proxy runs on the engine's default bridge instead.
	networkMode := container.NetworkMode(networkName)
	if hostOptions.NoProxy {
		networkMode = "bridge"
	}
	return &container.HostConfig{
		NetworkMode: networkMode,
		CapDrop:     []string{"ALL"},
		SecurityOpt: []string{"no-new-privileges"},
		Binds:       mountSpecs(hostOptions.Mounts),
		Tmpfs:       tmpfsMap(hostOptions.TmpfsPaths),
	}, nil
}

// Run starts the devbox container interactively (docker run -it --rm) behind the wall and
// returns its exit code. proxyStart brings the wall up for the session (see AutoProxy); the
// container is removed on return, before any wall proxyStart owns is torn down.
func Run(ctxD *dock.CtxD, hostOptions RunOptions) (int, error) {
	if hostOptions.NoProxy {
		return runDevbox(ctxD, hostOptions)
	}

	proxyRunning, err := isProxyRunning(ctxD)
	if err != nil {
		return 0, err
	}
	if proxyRunning {
		return runDevbox(ctxD, hostOptions)
	}
	return runWithProxy(ctxD, hostOptions)
}

func runWithProxy(ctxD *dock.CtxD, hostOptions RunOptions) (int, error) {
	file, err := os.OpenFile(hostOptions.ProxyLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, ioutil.File)
	if err != nil {
		return 0, err
	}
	defer log.Defer("close log file", file.Close)

	cleanup, err := proxyStart(ctxD, hostOptions.Proxy, func(logs io.ReadCloser) error {
		_, logErr := stdcopy.StdCopy(file, file, logs)
		return logErr
	})
	if err != nil {
		return 0, err
	}

	code, runErr := runDevbox(ctxD, hostOptions)
	return code, errors.Join(runErr, cleanup())
}

func runDevbox(ctxD *dock.CtxD, hostOptions RunOptions) (int, error) {
	if !hostOptions.NoProxy {
		_ = ctxD.D.NetworkConnect(ctxD.Ctx, "bridge", egressName, nil) // silently ignore errors
	}

	hostConfig, err := runHostConfig(ctxD, hostOptions)
	if err != nil {
		return 0, err
	}
	resp, err := ctxD.D.ContainerCreate(ctxD.Ctx, runConfig(hostOptions), hostConfig, nil, nil, "")
	if err != nil {
		return 0, err
	}

	// --rm: remove on return regardless of how we got here
	defer log.Defer("remove container", func() error {
		// context.Background() so a cancelled ctx can't block cleanup
		return ctxD.D.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
	})
	return dock.RunInteractive(ctxD, resp.ID)
}
