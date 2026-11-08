package docker

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"

	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/kit/tinyproxy"
	"github.com/s12chung/ccbox/pkg/util/errs"
	"github.com/s12chung/ccbox/pkg/util/klean"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/stream"
	"github.com/s12chung/ccbox/pkg/util/term"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/identity"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/flock"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/logmux"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/seccomputil"
)

// RunOptions configures the devbox run and its holder session.
type RunOptions struct {
	RunHostOptions

	Proxy *ProxyOptions // the run's egress wall; nil runs plain

	Name string // the devbox's fixed container name
	Tag  string
	Env  map[string]string // container env minus the proxy vars

	HoldersDir string      // the host dir of the devbox's holder flock entries
	LogFile    string      // the host file the devbox's own log lines append to
	Foreground *Foreground // the run's live view of the devbox's logs; nil for the exec sessions
}

// RunHostOptions is everything mounted into the devbox
type RunHostOptions struct {
	ProjectDir     string
	WorkspaceMount string
	Mounts         []Mount
	TmpfsPaths     []string
}

// Foreground streams the devbox's logs
type Foreground struct {
	Writer io.Writer
	Stop   chan struct{} // made by Run: closed when the devbox's log stream ends
}

func (o RunOptions) readyMarker() string {
	if o.Proxy != nil {
		return pkginfo.ReadyMarker
	}
	return pkginfo.DevboxReadyMarker
}

func (o RunOptions) matchesContainer(labels map[string]string) bool {
	return labels[ccboxProjectLabelKey] == o.ProjectDir &&
		(labels[ccboxHasProxyLabelKey] == trueVolValue) == (o.Proxy != nil)
}

func (o RunOptions) streamOptions() (stream.Options, error) {
	devboxFile, err := osutil.OpenAppend(o.LogFile)
	if err != nil {
		return stream.Options{}, err
	}
	var clean klean.Joiner
	clean.Push("close the devbox's log file", devboxFile.Close)

	opts := stream.Options{Writers: []io.Writer{devboxFile}, OnEnd: clean.Run}

	if o.Proxy != nil {
		proxyFile, err := osutil.OpenAppend(o.Proxy.LogFile)
		if err != nil {
			return stream.Options{}, errors.Join(err, clean.Run())
		}
		clean.Push("close the proxy's log file", proxyFile.Close)
		demuxer := &logmux.Demuxer{Writers: map[string]io.Writer{
			"":                   devboxFile,
			pkginfo.ProxyMuxName: proxyFile,
		}}
		clean.Push("file the stream's torn tail", demuxer.Flush)

		opts.Writers = []io.Writer{demuxer}
	}

	opts.ReadyWait = &stream.ReadyWait{Marker: o.readyMarker()}
	if o.Foreground == nil {
		opts.ReadyWait.Screen = term.NewColorWriter(os.Stdout, tinyproxy.LevelColor)
	}

	if o.Foreground != nil {
		opts.Writers = append(opts.Writers, term.NewColorWriter(o.Foreground.Writer, tinyproxy.LevelColor))
		clean.Push("close the run's foreground wait", func() error {
			close(o.Foreground.Stop) // closing Stop wakes the run's foreground wait
			return nil
		})
	}
	return opts, nil
}

// Run boots the devbox detached, streaming its logs to the foreground until interrupted
func Run(ctxD *dock.CtxD, o RunOptions) error {
	if o.Foreground == nil {
		return errors.New("the run needs a Foreground view: it streams the devbox's logs")
	}
	o.Foreground.Stop = make(chan struct{}) // the wait ends when the devbox's log stream does
	leave, err := holdDevbox(ctxD, o)
	if err != nil {
		return err
	}
	defer log.Defer("devbox leave", leave)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sig)
	select {
	case <-sig:
	case <-o.Foreground.Stop: // the stream ends with the container
	}
	return nil
}

// holdDevbox joins the devbox's holder session, ensuring the boot is up
func holdDevbox(ctxD *dock.CtxD, o RunOptions) (_ func() error, err error) {
	leave, err := flock.MultiFlock{Dir: o.HoldersDir}.Join(
		func() error { return verifyAndHeal(ctxD, o) },
		func() error { return createDevbox(ctxD, o) },
		func() error { return stopDevbox(ctxD, o) },
	)
	if err != nil {
		return nil, err
	}

	leave = klean.SwallowErr(leave, flock.ErrNotLast)
	defer func() { err = errs.JoinOnErr(err, leave) }()

	if err = dock.AwaitLogMarker(ctxD, o.Name, o.readyMarker()); err != nil {
		return nil, err
	}
	if o.Proxy != nil {
		if err = o.Proxy.OnRefresh(); err != nil {
			return nil, err
		}
	}
	if err = streamLogs(ctxD, o.Name, o.Foreground); err != nil {
		return nil, err
	}
	return leave, nil
}

// streamLogs tails the container's logs through fg in a goroutine, closing
// its Stop with the stream. A nil fg streams nothing.
func streamLogs(ctxD *dock.CtxD, name string, fg *Foreground) error {
	if fg == nil {
		return nil
	}
	logs, err := dock.FollowLogs(ctxD, name)
	if err != nil {
		return err
	}
	w := term.NewColorWriter(fg.Writer, tinyproxy.LevelColor)
	go func() {
		_, copyErr := io.Copy(w, logs)
		close(fg.Stop)
		log.WarnErr("close devbox logs", errors.Join(
			errs.Swallow(copyErr, io.ErrClosedPipe, os.ErrClosed, net.ErrClosed), logs.Close()))
	}()
	return nil
}

func verifyAndHeal(ctxD *dock.CtxD, o RunOptions) error {
	own, err := claimDevbox(ctxD, o)
	if err != nil {
		return err
	}
	if own == nil {
		return createDevbox(ctxD, o)
	}
	return nil
}

// createDevbox starts the devbox fresh, streaming its logs to the run's views;
// a failed create tears the half-boot down, leaving nothing behind
func createDevbox(ctxD *dock.CtxD, o RunOptions) (err error) {
	defer func() {
		err = errs.JoinOnErr(err, func() error { return stopDevbox(ctxD, o) })
	}()

	if err = startDevbox(ctxD, o); err != nil {
		return err
	}
	opts, err := o.streamOptions()
	if err != nil {
		return err
	}
	return dock.StreamLogs(ctxD, o.Name, opts)
}

// IsDevboxRunning reports whether the run's own devbox is running
func IsDevboxRunning(ctxD *dock.CtxD, o RunOptions) (bool, error) {
	own, err := claimDevbox(ctxD, o)
	return own != nil, err
}

// claimDevbox returns the run's own running devbox, nil when none is running —
// one running devbox per variant
func claimDevbox(ctxD *dock.CtxD, o RunOptions) (*container.InspectResponse, error) {
	own, err := dock.InspectRunning(ctxD, o.Name)
	if err = errs.Swallow(err, dock.ErrNotRunning); err != nil {
		return nil, err
	}
	if own != nil && !o.matchesContainer(own.Config.Labels) {
		return nil, conflictErr(o.Name, own.Config.Labels)
	}
	return own, nil
}

// conflictErr refuses the run: a devbox under the run's own name is already running
// with diverging labels — the error names it and how to clear it
func conflictErr(name string, labels map[string]string) error {
	proxy := "without the proxy"
	if labels[ccboxProjectLabelKey] == "" {
		proxy = "of an older ccbox, its proxy state unknown"
	} else if labels[ccboxHasProxyLabelKey] == trueVolValue {
		proxy = "with the proxy"
	}
	return fmt.Errorf("devbox %s is already running for this project, %s — exit its session or `docker rm -f %s`",
		name, proxy, name)
}

// startDevbox replaces any stale devbox with a fresh one off the run's options
func startDevbox(ctxD *dock.CtxD, o RunOptions) error {
	if err := dock.EnsureImageExists(ctxD, o.Tag); err != nil {
		return err
	}
	// Clear any stale devbox container so the fixed name is free.
	_ = ctxD.D.ContainerRemove(ctxD.Ctx, o.Name, container.RemoveOptions{Force: true})

	if o.Proxy != nil {
		if err := o.Proxy.BeforeStart(); err != nil {
			return err
		}
	}

	hostConfig, err := runHostConfig(ctxD, o)
	if err != nil {
		return err
	}
	resp, err := ctxD.D.ContainerCreate(ctxD.Ctx, runConfig(o), hostConfig, nil, nil, o.Name)
	if err != nil {
		return err
	}
	return ctxD.D.ContainerStart(ctxD.Ctx, resp.ID, container.StartOptions{})
}

const (
	// desktopVNCPort is the port a run's desktop serves VNC on — the stock RFB port;
	// macOS Screen Sharing only takes it without the display number
	desktopVNCPort = "5900"
	// ipForwardSysctl pins the devbox netns's forwarding off; the container's /proc/sys is read-only
	ipForwardSysctl = "net.ipv4.ip_forward"
)

// refuseHeldPorts is a var so tests can stub it.
var refuseHeldPorts = dock.RefuseHeldPorts

func runHostConfig(ctxD *dock.CtxD, hostOptions RunOptions) (*container.HostConfig, error) {
	if err := ensureMounts(ctxD, hostOptions.Tag, hostOptions.ProjectDir, hostOptions.Mounts); err != nil {
		return nil, err
	}
	seccompOpt, err := seccomputil.BootOpt()
	if err != nil {
		return nil, err
	}

	hostConfig := &container.HostConfig{
		CapDrop:     []string{"ALL"},
		SecurityOpt: append([]string{"no-new-privileges"}, seccompOpt),
		Binds:       mountSpecs(hostOptions.Mounts),
		Tmpfs:       tmpfsMap(hostOptions.TmpfsPaths),
	}
	if hostOptions.Proxy == nil {
		hostConfig.PortBindings = nat.PortMap{desktopVNCPort + "/tcp": {{HostIP: "127.0.0.1", HostPort: desktopVNCPort}}}
	} else {
		hostConfig.CapAdd = identity.RunCapNames()
		hostConfig.Sysctls = map[string]string{ipForwardSysctl: "0"}
	}
	if err := refuseHeldPorts(ctxD, hostConfig.PortBindings); err != nil {
		return nil, err
	}
	return hostConfig, nil
}

func runConfig(hostOptions RunOptions) *container.Config {
	cfg := &container.Config{
		Image:      hostOptions.Tag,
		Labels:     map[string]string{ccboxProjectLabelKey: hostOptions.ProjectDir},
		WorkingDir: hostOptions.WorkspaceMount,
		Env:        envString(hostOptions.Env, hostOptions.Proxy == nil),
	}
	if hostOptions.Proxy != nil {
		// the egress wall's devbox stays root to build, prove, and supervise tinyproxy —
		// workloads still arrive dropped, via the drop chain.
		//
		// Raw engine exec lands on this root too; the devbox can't refuse it.
		cfg.User = "root"
		cfg.Labels[ccboxHasProxyLabelKey] = trueVolValue
	}
	return cfg
}

// stopDevbox tears the devbox down detached, so a cancelled caller's ctx can't block
// the cleanups; already-clean pieces are no error
func stopDevbox(ctxD *dock.CtxD, o RunOptions) error {
	ctxD = ctxD.BackgroundedCtx()
	timeout := 5
	err := ctxD.D.ContainerStop(ctxD.Ctx, o.Name, container.StopOptions{Timeout: &timeout})
	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	err = ctxD.D.ContainerRemove(ctxD.Ctx, o.Name, container.RemoveOptions{Force: true})
	if err != nil && !errdefs.IsNotFound(err) {
		return err
	}
	if o.Proxy != nil {
		return o.Proxy.OnStop()
	}
	return nil
}
