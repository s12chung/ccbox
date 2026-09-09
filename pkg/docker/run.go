package docker

import (
	"context"
	"errors"
	"io"
	"maps"
	"os"
	"path"
	"slices"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/s12chung/ccbox/ccboxtools/pkg/log"
	"github.com/s12chung/ccbox/pkg/harness"
	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
)

const proxyPort = "8888"

// RunOptions configures the interactive devbox container.
type RunOptions struct {
	Tag             string
	CLI             string            // selects the config mount target (each CLI's native default dir)
	CLIConfigDir    string            // host cliConfigDir bind-mounted at the cliCOnfigMount
	ProjectStateDir string            // host projectStateDir bind-mounted at projectStateMount
	Cwd             string            // host project dir bind-mounted at workspaceMount
	CLIDataBinds    map[string]string // host path → $HOME-relative in-container path, rw bound under containerHome
	AgentsMdBind    string            // host file rw-bound as the CLI's SeedAgentsFilename; "" = the CLI has its own
	GitConfigDir    string            // host ~/.config/git bind-mounted read-only at gitConfigMount; "" = skip
	GHToken         string            // GH_TOKEN passed through for gh
	Env             map[string]string // extra container env
	TmpfsMasks      []string          // project-relative dirs to mask with a temp filesystem
	VolumeMasks     []string          // project-relative dirs to mask with a persistent per-project volume
	ReadOnlyPaths   []string          // project-relative paths to re-mount read-only
	Cmd             []string          // command the entrypoint execs; nil uses the image default (shell)

	Proxy        ProxyOptions // configs + generated allow.txt for an auto-started wall
	ProxyLogPath string       // file an auto-started wall's logs are appended to
	NoProxy      bool         // run on plain bridge networking, no wall or proxy env
}

const (
	containerHome     = "/home/ccbox"                // mounts sit under containerHome at a per-project leaf
	projectStateMount = "/home/ccbox/.ccbox/project" // per-project devbox state (e.g. lessons)

	gitConfigMount = "/home/ccbox/.config/git" // host global git dir, read-only (git's default XDG path)
)

func runConfig(hostOptions RunOptions) *container.Config {
	return &container.Config{
		Image:        hostOptions.Tag,
		Cmd:          hostOptions.Cmd,
		WorkingDir:   workspaceMount(hostOptions.Cwd),
		Tty:          true,
		OpenStdin:    true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Env:          envString(hostOptions),
	}
}

func runHostConfig(ctxD *dock.CtxD, hostOptions RunOptions) (*container.HostConfig, error) {
	globalBinds, err := ensureVolumes(ctxD, globalVolumes, globalVolumeLabels())
	if err != nil {
		return nil, err
	}
	cacheMounts, err := ensureVolumes(ctxD, cacheVolumeNames(hostOptions.Cwd), projectVolumeLabels(hostOptions.Cwd))
	if err != nil {
		return nil, err
	}

	tmpfsMounts, err := tmpfsMasks(hostOptions.Cwd, hostOptions.TmpfsMasks)
	if err != nil {
		return nil, err
	}
	volumeMaskMounts, err := ensureNamedVolumeMasks(ctxD, hostOptions.Cwd, hostOptions.Tag, hostOptions.VolumeMasks)
	if err != nil {
		return nil, err
	}
	roPathBinds, err := readOnlyPathBinds(hostOptions.Cwd, hostOptions.ReadOnlyPaths)
	if err != nil {
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
		Binds: slices.Concat([]string{
			hostOptions.Cwd + ":" + workspaceMount(hostOptions.Cwd),
			hostOptions.CLIConfigDir + ":" + path.Join(containerHome, harness.MustFor(hostOptions.CLI).ConfigHomeMount),
			hostOptions.ProjectStateDir + ":" + projectStateMount,
		}, globalBinds, cacheMounts, volumeMaskMounts, roPathBinds,
			gitBinds(hostOptions.GitConfigDir), cliDataBinds(hostOptions.CLIDataBinds),
			agentsMdBind(hostOptions)),
		Tmpfs: tmpfsMounts,
	}, nil
}

func gitBinds(gitConfigDir string) []string {
	if gitConfigDir == "" {
		return nil
	}
	return []string{gitConfigDir + ":" + gitConfigMount + ":ro"}
}

// cliDataBinds renders CLIDataBinds as host:container rw binds, sorted for a deterministic spec.
func cliDataBinds(m map[string]string) []string {
	binds := make([]string, 0, len(m))
	for _, host := range slices.Sorted(maps.Keys(m)) {
		binds = append(binds, host+":"+path.Join(containerHome, m[host]))
	}
	return binds
}

// agentsMdBind binds o.AgentsMdBind to the container's CLI config path.
// If o.AgentsMdBind is empty, skips (because a SeedAgentsFilename already exists)
func agentsMdBind(o RunOptions) []string {
	if o.AgentsMdBind == "" {
		return nil
	}
	cli := harness.MustFor(o.CLI)
	return []string{o.AgentsMdBind + ":" + path.Join(containerHome, cli.ConfigHomeMount, cli.SeedAgentsFilename)}
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
