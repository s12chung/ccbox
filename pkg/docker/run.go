package docker

import (
	"context"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"

	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
)

const proxyPort = "8888"

// bridgeNetworkName is the engine's default bridge: the devbox runs on the internal
// proxy network, so the proxy itself forwards over the bridge.
const bridgeNetworkName = "bridge"

// desktopVNCPort is the port a run's desktop serves native VNC on. Published
// on the host loopback, so a VNC client connects at localhost:5900 when the run
// serves a desktop (--vnc); nothing listens behind the port when it doesn't. 5900
// is the stock RFB port, and macOS Screen Sharing only takes it without the
// display number.
const desktopVNCPort = "5900"

// RunOptions configures the interactive devbox container.
type RunOptions struct {
	RunHostOptions

	Tag string
	Env map[string]string // container env minus the proxy vars
	Cmd []string          // command the entrypoint execs; nil uses the image default (shell)

	// Proxy is the auto-started egress wall's options; nil runs on plain bridge
	// networking: no proxy network or env
	Proxy *ProxyOptions
}

// RunHostOptions is everything mounted into the devbox: the workspace, the host dirs
// and volumes as mounts, and the project dirs masked with tmpfs. ProjectDir is the project
// identity labeling its volumes; WorkspaceMount is the container's WorkingDir.
type RunHostOptions struct {
	ProjectDir     string
	WorkspaceMount string
	Mounts         []Mount
	TmpfsPaths     []string
}

func runConfig(hostOptions RunOptions) *container.Config {
	return &container.Config{
		Image:        hostOptions.Tag,
		Cmd:          hostOptions.Cmd,
		WorkingDir:   hostOptions.WorkspaceMount,
		Tty:          true,
		OpenStdin:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Env:          envString(hostOptions.Env, hostOptions.Proxy == nil),
	}
}

func runHostConfig(ctxD *dock.CtxD, hostOptions RunOptions) (*container.HostConfig, error) {
	if err := ensureMounts(ctxD, hostOptions.Tag, hostOptions.ProjectDir, hostOptions.Mounts); err != nil {
		return nil, err
	}

	// The egress wall network by default; a nil proxy runs on the engine's default bridge instead.
	networkMode := container.NetworkMode(proxyNetworkName)
	if hostOptions.Proxy == nil {
		networkMode = bridgeNetworkName
	}
	return &container.HostConfig{
		NetworkMode: networkMode,
		CapDrop:     []string{"ALL"},
		SecurityOpt: []string{"no-new-privileges"},
		// Loopback-only: native RFB is VncAuth-password auth on a
		// plain wire, so keep it off the network.
		PortBindings: nat.PortMap{desktopVNCPort + "/tcp": {{HostIP: "127.0.0.1", HostPort: desktopVNCPort}}},
		Binds:        mountSpecs(hostOptions.Mounts),
		Tmpfs:        tmpfsMap(hostOptions.TmpfsPaths),
	}, nil
}

// Run starts the devbox container interactively (docker run -it --rm) behind the egress wall
// and returns its exit code. holdProxy holds the egress wall's session for the run — the proxy
// outlives the run whenever another holder remains — and the container is removed on
// return, before the session's leave.
func Run(ctxD *dock.CtxD, hostOptions RunOptions) (int, error) {
	if hostOptions.Proxy == nil {
		return runDevbox(ctxD, hostOptions)
	}
	leave, err := holdProxy(ctxD, *hostOptions.Proxy)
	if err != nil {
		return 0, err
	}
	defer log.Defer("proxy leave", leave)
	return runDevbox(ctxD, hostOptions)
}

func runDevbox(ctxD *dock.CtxD, hostOptions RunOptions) (int, error) {
	if hostOptions.Proxy != nil {
		// re-asserted per run: a proxy adopted from a creator that skipped it gets egress here
		_ = ctxD.D.NetworkConnect(ctxD.Ctx, bridgeNetworkName, proxyContainerName, nil) // silently ignore errors
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
