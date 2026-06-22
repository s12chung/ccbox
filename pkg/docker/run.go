package docker

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"

	"github.com/s12chung/ccbox/pkg/log"
	"github.com/s12chung/ccbox/pkg/prompt"
)

const proxyPort = "8888"

// RunOptions configures the interactive devbox container.
type RunOptions struct {
	Tag          string
	ConfigDir    string // host dir bind-mounted at configMount
	CcboxDir     string // host dir bind-mounted at ccboxMount
	WorkspaceDir string // host dir bind-mounted at workspaceMount
	OAuthToken   string
	GHToken      string
}

const (
	configMount   = "/home/ccbox/.ccbox/claude-config" // configMount is threaded into the build (CLAUDE_CONFIG_DIR arg)
	ccboxMount    = "/home/ccbox/.ccbox/project"       // per-project devbox state (e.g. lessons)
	containerHome = "/home/ccbox"                      // the workspace mounts under containerHome at a per-project leaf
)

// ContainerMount is the in-container workspace, so per-project dirs (in claude-config, .ccbox) are keyed by it, not shared.
func ContainerMount(workspaceDir string) string {
	return filepath.Join(containerHome, filepath.Base(workspaceDir))
}

// Run starts the devbox container interactively (docker run -it --rm) behind the
// wall and returns its exit code. The container is removed on return.
func (c *Client) Run(ctx context.Context, o RunOptions) (int, error) {
	if err := c.ensureProxyRunning(ctx); err != nil {
		return 0, err
	}
	_ = c.cli.NetworkConnect(ctx, "bridge", egressName, nil) // silently ignore errors

	resp, err := c.cli.ContainerCreate(ctx,
		&container.Config{
			Image:        o.Tag,
			WorkingDir:   ContainerMount(o.WorkspaceDir),
			Tty:          true,
			OpenStdin:    true,
			AttachStdin:  true,
			AttachStdout: true,
			AttachStderr: true,
			Env: []string{
				"http_proxy=http://" + egressName + ":" + proxyPort,
				"https_proxy=http://" + egressName + ":" + proxyPort,
				"CLAUDE_CODE_OAUTH_TOKEN=" + o.OAuthToken,
				"GH_TOKEN=" + o.GHToken,
			},
		},
		&container.HostConfig{
			NetworkMode: networkName,
			CapDrop:     []string{"ALL"},
			SecurityOpt: []string{"no-new-privileges"},
			Binds: []string{
				o.ConfigDir + ":" + configMount,
				o.CcboxDir + ":" + ccboxMount,
				o.WorkspaceDir + ":" + ContainerMount(o.WorkspaceDir),
			},
			Tmpfs: map[string]string{ContainerMount(o.WorkspaceDir) + "/.idea": ""},
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

// ensureProxyRunning fails fast with errProxyDown unless the egress container is
// up — a missing or stopped wall can't carry any traffic.
func (c *Client) ensureProxyRunning(ctx context.Context) error {
	errProxyDown := errors.New("egress proxy not running; start it in another terminal with `ccbox proxy`")

	info, err := c.cli.ContainerInspect(ctx, egressName)
	if errdefs.IsNotFound(err) {
		return errProxyDown
	}
	if err != nil {
		return err
	}
	if !info.State.Running {
		return errProxyDown
	}
	return nil
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
