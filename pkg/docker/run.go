package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/s12chung/ccbox/pkg/log"
	"github.com/s12chung/ccbox/pkg/perm"
	"github.com/s12chung/ccbox/pkg/prompt"
)

const proxyPort = "8888"

// containerUID is the unprivileged in-container user (Dockerfile: useradd --uid 1000).
const containerUID = "1000"

// tmpfsOpts makes a workspace tmpfs writable+executable by that user, so masked build
// outputs (e.g. dist/) can be written and run — Docker's default is root-owned noexec.
var tmpfsOpts = fmt.Sprintf("uid=%s,gid=%s,exec", containerUID, containerUID)

// RunOptions configures the interactive devbox container.
type RunOptions struct {
	Tag        string
	ConfigDir  string // host dir bind-mounted at configMount
	CcboxDir   string // host dir bind-mounted at ccboxMount
	Cwd        string // host dir bind-mounted at workspaceMount
	OAuthToken string
	GHToken    string
	Env        map[string]string // extra container env
	Tmpfs      []string          // workspace-relative dirs to mask

	AutoProxy    bool   // start (and tear down) the egress wall for this run; see ProxyWrap
	ProxyConfig  fs.FS  // tinyproxy configs, for an auto-started wall
	ProxyLogPath string // file an auto-started wall's logs are appended to
}

const (
	configMount   = "/home/ccbox/.ccbox/claude-config" // configMount is threaded into the build (CLAUDE_CONFIG_DIR arg)
	ccboxMount    = "/home/ccbox/.ccbox/project"       // per-project devbox state (e.g. lessons)
	containerHome = "/home/ccbox"                      // the workspace mounts under containerHome at a per-project leaf
)

// WorkspaceMount is the in-container workspace path: the WorkingDir and bind target for the host cwd.
func WorkspaceMount(hostCwd string) string {
	return filepath.Join(containerHome, filepath.Base(hostCwd))
}

// ProjectSlug is ccbox's per-project key: the host cwd slugified (e.g. /Users/me/app → -Users-me-app).
// Distinct from Claude Code's claude-config/projects slug, which CC derives from its container cwd.
func ProjectSlug(hostCwd string) string {
	return strings.ReplaceAll(hostCwd, "/", "-")
}

// cacheVolumes maps suffix of volume name → container directory for cacheVolumeBinds()
var cacheVolumes = map[string]string{
	"go":         "/home/ccbox/go",          // go mod tidy module cache + GOBIN
	"cache":      "/home/ccbox/.cache",      // go-build + pip cache
	"gem":        "/home/ccbox/.gem",        // bundler GEM_HOME
	"npm":        "/home/ccbox/.npm",        // npm download cache
	"npm-global": "/home/ccbox/.npm-global", // global npm packages
	"local":      "/home/ccbox/.local",      // pip --user installs
}

// cacheVolumeName is hostCwd's named volume for a given cacheVolumes suffix.
func cacheVolumeName(hostCwd, suffix string) string {
	return "ccbox" + ProjectSlug(hostCwd) + "-" + suffix
}

// cacheVolumeBinds returns the volume binds from cacheVolumes, not mounted for efficiency of small files
func cacheVolumeBinds(hostCwd string) []string {
	binds := make([]string, 0, len(cacheVolumes))
	for suffix, dir := range cacheVolumes {
		binds = append(binds, cacheVolumeName(hostCwd, suffix)+":"+dir)
	}
	sort.Strings(binds)
	return binds
}

// VolumeClean removes hostCwd's cache volumes. An already-gone volume is skipped;
// other errors (e.g. still in use by a running devbox) are joined and returned.
func (c *Client) VolumeClean(ctx context.Context, hostCwd string) error {
	var errs []error
	for suffix := range cacheVolumes {
		if err := c.cli.VolumeRemove(ctx, cacheVolumeName(hostCwd, suffix), false); err != nil && !errdefs.IsNotFound(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// buildEnv renders base + extra as KEY=VALUE. extra goes first so base wins on a key
// collision (Docker takes the last value), keeping the proxy/token vars unoverridable.
func buildEnv(base []string, extra map[string]string) []string {
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys) // stable order for a deterministic spec

	env := make([]string, 0, len(extra)+len(base))
	for _, k := range keys {
		env = append(env, k+"="+extra[k])
	}
	return append(env, base...)
}

// buildTmpfs maps each workspace-relative path from .ccbox.yaml to its tmpfs options
// (writable+exec). Paths must stay inside the workspace, so absolute or ..-escaping
// ones are rejected.
func buildTmpfs(workspaceMount string, paths []string) (map[string]string, error) {
	tmpfs := map[string]string{}
	for _, p := range paths {
		dest := filepath.Join(workspaceMount, p)
		if rel, err := filepath.Rel(workspaceMount, dest); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("tmpfs path escapes workspace: %q", p)
		}
		tmpfs[dest] = tmpfsOpts
	}
	return tmpfs, nil
}

// Run starts the devbox container interactively (docker run -it --rm) behind the wall and
// returns its exit code. proxyWrap brings the wall up for the session (see AutoProxy); the
// container is removed on return, before any wall proxyWrap owns is torn down.
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

	var code int
	logDone, err := c.proxyWrap(ctx, hostOptions.ProxyConfig, func() error {
		var runErr error
		code, runErr = c.runDevbox(ctx, hostOptions)
		return runErr
	}, func(logs io.ReadCloser) error {
		_, logErr := stdcopy.StdCopy(file, file, logs)
		return logErr
	})

	var logErr error
	if logDone != nil {
		logErr = <-logDone
	}
	return code, errors.Join(err, logErr)
}

func (c *Client) runDevbox(ctx context.Context, hostOptions RunOptions) (int, error) {
	_ = c.cli.NetworkConnect(ctx, "bridge", egressName, nil) // silently ignore errors

	baseEnv := []string{
		"http_proxy=http://" + egressName + ":" + proxyPort,
		"https_proxy=http://" + egressName + ":" + proxyPort,
		"CLAUDE_CODE_OAUTH_TOKEN=" + hostOptions.OAuthToken,
		"GH_TOKEN=" + hostOptions.GHToken,
	}
	workspaceMount := WorkspaceMount(hostOptions.Cwd)
	tmpfs, err := buildTmpfs(workspaceMount, hostOptions.Tmpfs)
	if err != nil {
		return 0, err
	}

	resp, err := c.cli.ContainerCreate(ctx,
		&container.Config{
			Image:        hostOptions.Tag,
			WorkingDir:   workspaceMount,
			Tty:          true,
			OpenStdin:    true,
			AttachStdin:  true,
			AttachStdout: true,
			AttachStderr: true,
			Env:          buildEnv(baseEnv, hostOptions.Env),
		},
		&container.HostConfig{
			NetworkMode: networkName,
			CapDrop:     []string{"ALL"},
			SecurityOpt: []string{"no-new-privileges"},
			Binds: append([]string{
				hostOptions.ConfigDir + ":" + configMount,
				hostOptions.CcboxDir + ":" + ccboxMount,
				hostOptions.Cwd + ":" + workspaceMount,
			}, cacheVolumeBinds(hostOptions.Cwd)...),
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
