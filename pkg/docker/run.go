package docker

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/s12chung/ccbox/pkg/log"
	"github.com/s12chung/ccbox/pkg/perm"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/prompt"
)

const proxyPort = "8888"

// RunOptions configures the interactive devbox container.
type RunOptions struct {
	Tag          string
	CLI          projectcfg.CLI    // selects the config mount target (each CLI's native default dir)
	ConfigDir    string            // host dir bind-mounted at the CLI's configMount
	CcboxDir     string            // host dir bind-mounted at ccboxMount
	Cwd          string            // host dir bind-mounted at workspaceMount
	GitConfigDir string            // host ~/.config/git bind-mounted read-only at gitConfigMount; "" = skip
	GHToken      string            // GH_TOKEN passed through for gh
	Env          map[string]string // extra container env
	Tmpfs        []string          // workspace-relative dirs to mask with an ephemeral tmpfs
	Volumes      []string          // workspace-relative dirs to mask with a persistent per-project volume
	Cmd          []string          // command the entrypoint execs; nil uses the image default (shell)

	AutoProxy    bool         // start (and tear down) the egress wall for this run; see proxyStart
	Proxy        ProxyOptions // configs + generated allow.txt for an auto-started wall
	ProxyLogPath string       // file an auto-started wall's logs are appended to
}

const (
	containerHome = "/home/ccbox"                // the workspace mounts under containerHome at a per-project leaf
	ccboxMount    = "/home/ccbox/.ccbox/project" // per-project devbox state (e.g. lessons)

	// Per-CLI config mounts: each CLI's native default dir, so the mounted config is found with
	// no CLAUDE_CONFIG_DIR/CODEX_HOME override. Threaded into the build too (CONFIG_DIR arg).
	claudeConfigMount = containerHome + "/.claude"
	codexConfigMount  = containerHome + "/.codex"

	gitConfigMount = "/home/ccbox/.config/git" // host global git dir, read-only (git's default XDG path)
)

// configMount is the in-container path the persisted config dir binds to for cli.
func configMount(cli projectcfg.CLI) string {
	if cli == projectcfg.CLICodex {
		return codexConfigMount
	}
	return claudeConfigMount
}

// WorkspaceMount is the in-container workspace path: the WorkingDir and bind target for the host cwd.
func WorkspaceMount(hostCwd string) string {
	return filepath.Join(containerHome, filepath.Base(hostCwd))
}

// ProjectSlug is ccbox's per-project key: the host cwd slugified (e.g. /Users/me/app → -Users-me-app).
// Distinct from Claude Code's .claude/projects slug, which CC derives from its container cwd.
func ProjectSlug(hostCwd string) string {
	return strings.ReplaceAll(hostCwd, "/", "-")
}

// Run starts the devbox container interactively (docker run -it --rm) behind the wall and
// returns its exit code. proxyStart brings the wall up for the session (see AutoProxy); the
// container is removed on return, before any wall proxyStart owns is torn down.
func (c *Client) Run(ctx context.Context, hostOptions RunOptions) (int, error) {
	running, err := c.proxyRunning(ctx)
	if err != nil {
		return 0, err
	}
	if running {
		return c.runDevbox(ctx, hostOptions)
	} else if !hostOptions.AutoProxy {
		return 0, errors.New("egress proxy not running; start it in another terminal with `ccbox proxy`")
	}

	return c.runWithProxy(ctx, hostOptions)
}

func (c *Client) runWithProxy(ctx context.Context, hostOptions RunOptions) (int, error) {
	file, err := os.OpenFile(hostOptions.ProxyLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, perm.File)
	if err != nil {
		return 0, err
	}
	defer log.Defer("close log file", file.Close)

	cleanup, err := c.proxyStart(ctx, hostOptions.Proxy, func(logs io.ReadCloser) error {
		_, logErr := stdcopy.StdCopy(file, file, logs)
		return logErr
	})
	if err != nil {
		return 0, err
	}

	code, runErr := c.runDevbox(ctx, hostOptions)
	return code, errors.Join(runErr, cleanup())
}

func (c *Client) runDevbox(ctx context.Context, hostOptions RunOptions) (int, error) {
	_ = c.cli.NetworkConnect(ctx, "bridge", egressName, nil) // silently ignore errors

	tmpfs, err := tmpfsMasks(hostOptions.Cwd, hostOptions.Tmpfs)
	if err != nil {
		return 0, err
	}
	volumeMaskBinds, err := c.ensureNamedVolumeMasks(ctx, hostOptions.Cwd, hostOptions.Tag, hostOptions.Volumes)
	if err != nil {
		return 0, err
	}
	cacheBinds, err := c.ensureCacheVolumes(ctx, hostOptions.Cwd)
	if err != nil {
		return 0, err
	}
	var gitBinds []string
	if hostOptions.GitConfigDir != "" {
		gitBinds = []string{hostOptions.GitConfigDir + ":" + gitConfigMount + ":ro"}
	}
	resp, err := c.cli.ContainerCreate(ctx,
		&container.Config{
			Image:        hostOptions.Tag,
			Cmd:          hostOptions.Cmd,
			WorkingDir:   WorkspaceMount(hostOptions.Cwd),
			Tty:          true,
			OpenStdin:    true,
			AttachStdin:  true,
			AttachStdout: true,
			AttachStderr: true,
			Env:          envString(hostOptions),
		},
		&container.HostConfig{
			NetworkMode: networkName,
			CapDrop:     []string{"ALL"},
			SecurityOpt: []string{"no-new-privileges"},
			Binds: append(append(append([]string{
				hostOptions.ConfigDir + ":" + configMount(hostOptions.CLI),
				hostOptions.CcboxDir + ":" + ccboxMount,
				hostOptions.Cwd + ":" + WorkspaceMount(hostOptions.Cwd),
			}, cacheBinds...), volumeMaskBinds...), gitBinds...),
			Tmpfs: tmpfs,
		},
		nil, nil, "")
	if err != nil {
		return 0, err
	}

	// --rm: remove on return regardless of how we got here
	defer log.Defer("remove container", func() error {
		// context.Background() so a cancelled ctx can't block cleanup
		return c.cli.ContainerRemove(context.Background(), resp.ID, container.RemoveOptions{Force: true})
	})
	return c.runInteractive(ctx, resp.ID)
}

// proxyRunning reports whether the egress container is up. A missing or stopped wall is
// false, not an error.
func (c *Client) proxyRunning(ctx context.Context) (bool, error) {
	info, err := c.cli.ContainerInspect(ctx, egressName)
	if errdefs.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.State.Running, nil
}

// runInteractive wires the local terminal to the container: raw mode, a hijacked
// bidirectional stream, and SIGWINCH-driven resize. docker (the daemon) owns the
// pty; we only shuttle bytes and window sizes.
//
// Discipline: every exit path must restore the terminal, so this returns errors
// rather than calling os.Exit/log.Fatal (which skip defers). On a kill/hangup it
// stops the container instead of exiting, so this same unwind still runs.
func (c *Client) runInteractive(ctx context.Context, id string) (int, error) {
	att, err := c.cli.ContainerAttach(ctx, id, container.AttachOptions{
		Stream: true, Stdin: true, Stdout: true, Stderr: true,
	})
	if err != nil {
		return 0, err
	}
	defer att.Close()

	if restore, ok := prompt.RawTerminal(); ok {
		defer log.Defer("restore terminal", restore)
	}

	// Register the wait before start so a fast exit isn't missed. NextExit (not
	// NotRunning) is essential: a created, not-yet-started container already satisfies
	// "not running", so NotRunning returns immediately with a bogus exit code 0.
	statusCh, errCh := c.cli.ContainerWait(ctx, id, container.WaitConditionNextExit)

	if err := c.cli.ContainerStart(ctx, id, container.StartOptions{}); err != nil {
		return 0, err
	}

	// Forward window resizes to the container's tty.
	stopResizes := prompt.ForwardResizes(func(h, w uint) {
		_ = c.cli.ContainerResize(ctx, id, container.ResizeOptions{Height: h, Width: w})
	})
	defer stopResizes()

	// On a kill/hangup, stop the container; that EOFs the output copy below, so the
	// normal unwind restores the terminal and (via Run's defer) removes the container.
	// Raw-mode Ctrl-C already goes to the container, so SIGINT here only fires for
	// non-tty stdin / `kill -INT`.
	kill := make(chan os.Signal, 1)
	signal.Notify(kill, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)
	defer signal.Stop(kill)
	go func() {
		<-kill
		timeout := 5
		_ = c.cli.ContainerStop(context.Background(), id, container.StopOptions{Timeout: &timeout})
	}()

	// Stdin → container (leaks on the os.Stdin read at process exit — fine for a CLI).
	go func() {
		_, _ = io.Copy(att.Conn, os.Stdin)
		_ = att.CloseWrite()
	}()
	// Container → stdout (tty merges stdout+stderr). Blocks until the container exits,
	// flushing all output before the deferred terminal restore.
	_, _ = io.Copy(os.Stdout, att.Reader)

	select {
	case err := <-errCh:
		return 0, err
	case st := <-statusCh:
		return int(st.StatusCode), nil
	}
}
