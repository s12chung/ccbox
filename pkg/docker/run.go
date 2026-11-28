package docker

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"

	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/kit/tinyproxy"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/klean"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/slicex"
	"github.com/s12chung/ccbox/pkg/util/stream"
	"github.com/s12chung/ccbox/pkg/util/term"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/identity"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/errs"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/flock"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/log"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/logmux"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/seccomputil"
)

// RunOptions configures the devbox run and its holder session.
type RunOptions struct {
	RunHostOptions

	Proxy *ProxyOptions // the run's egress wall; nil runs plain

	Name     string // the devbox's fixed container name
	Tag      string
	Env      map[string]string // container env minus the proxy vars
	ServeVNC bool

	// PortBindingsTCP publishes each port on localhost, host port = container port,
	// beside the desktop's own binding
	PortBindingsTCP []string

	// HostGatewayTCP names the host tcp ports the devbox may reach at host.docker.internal
	HostGatewayTCP []string

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

	opts.ReadyWait = &stream.ReadyWait{Marker: pkginfo.ReadyMarker}
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

	// mark is never held without its run's holder entry — otherwise, ps reads Execs as -1
	release, err := takeForegroundMark(o)
	if err != nil {
		return err
	}
	defer log.Defer("release the foreground mark", release)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
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

	flockLeave := klean.SwallowErr(leave, flock.ErrNotLast)
	defer func() { err = errs.JoinOnErr(err, flockLeave) }()

	// leave is idempotent: the session's end and its kill/hangup both fire it
	var once sync.Once
	leave = func() error {
		var leaveErr error
		once.Do(func() { leaveErr = flockLeave() })
		return leaveErr
	}

	if err = dock.AwaitLogMarker(ctxD, o.Name, pkginfo.ReadyMarker); err != nil {
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

// takeForegroundMark takes the devbox's foreground mark — only one foreground
// run streams its logs
func takeForegroundMark(o RunOptions) (func() error, error) {
	mark, err := flock.TryEx(foregroundMarkPath(o.HoldersDir))
	if errors.Is(err, flock.ErrHeld) {
		return nil, fmt.Errorf("devbox %s is already in the foreground — tail its log file instead: %s",
			o.Name, userdir.Tilde(o.LogFile))
	}
	if err != nil {
		return nil, err
	}
	return mark.Release, nil
}

// foregroundMarkPath is the foreground mark's file — the holder sweep counts
// only holder- files, so the mark sits there uncounted
func foregroundMarkPath(holdersDir string) string { return filepath.Join(holdersDir, "foreground") }

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

// IsAnyDevboxRunning reports whether any of the named devboxes is running
func IsAnyDevboxRunning(ctxD *dock.CtxD, names []string) (bool, error) {
	for _, name := range names {
		_, err := dock.InspectRunning(ctxD, name)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, dock.ErrNotRunning) {
			return false, err
		}
	}
	return false, nil
}

// claimDevbox returns the run's own running devbox, nil when none is running —
// one running devbox per variant
func claimDevbox(ctxD *dock.CtxD, o RunOptions) (*container.InspectResponse, error) {
	own, err := dock.InspectRunning(ctxD, o.Name)
	if err = errs.Swallow(err, dock.ErrNotRunning); err != nil {
		return nil, err
	}
	if own != nil {
		if err = o.conflictErr(own); err != nil {
			return nil, err
		}
	}
	return own, nil
}

// conflictErr refuses the run when a devbox under the run's own name is already
// running off diverging flags + config
func (o RunOptions) conflictErr(own *container.InspectResponse) error {
	config, err := pkginfo.RunConfigFromJSON(o.Env[pkginfo.RunConfigEnvVar])
	if err != nil {
		return fmt.Errorf("run env: %w", err)
	}
	info, err := pkginfo.FromJSON(o.Env[pkginfo.EnvVar])
	if err != nil {
		return fmt.Errorf("run env: %w", err)
	}

	env := own.Config.Env
	installedConfig, err := pkginfo.RunConfigFromJSON(dock.EnvEntry(env, pkginfo.RunConfigEnvVar))
	if err != nil {
		return fmt.Errorf("devbox env: %w", err)
	}
	installedInfo, err := pkginfo.FromJSON(dock.EnvEntry(env, pkginfo.EnvVar))
	if err != nil {
		return fmt.Errorf("devbox env: %w", err)
	}

	divergences := installedAsks(own, installedConfig, installedInfo).divergencesFrom(o.asks(config, info))
	if len(divergences) > 0 {
		return fmt.Errorf("devbox %s is already running for this project, %s — exit its session or `docker rm -f %s`",
			o.Name, strings.Join(divergences, " and "), o.Name)
	}
	return nil
}

// devboxAsks are what a run asks of its devbox, which a running one must match to be reused
type devboxAsks struct {
	projectDir   string
	hasProxy     bool
	portBindings nat.PortMap
	cliName      string
	cliVersion   string
	gatewayTCP   []string // sorted: a port list's order carries no meaning
}

// installedAsks reads a running devbox's asks off its inspect
func installedAsks(own *container.InspectResponse, config pkginfo.RunConfig, info pkginfo.PkgInfo) devboxAsks {
	labels := own.Config.Labels
	return devboxAsks{
		projectDir:   labels[ccboxProjectLabelKey],
		hasProxy:     labels[ccboxHasProxyLabelKey] == trueVolValue,
		portBindings: own.HostConfig.PortBindings,
		cliName:      info.Name,
		cliVersion:   config.CLIVersion,
		gatewayTCP:   slicex.SortNumbers(config.HostGatewayTCP),
	}
}

// asks renders the run's own asks
func (o RunOptions) asks(config pkginfo.RunConfig, info pkginfo.PkgInfo) devboxAsks {
	return devboxAsks{
		projectDir:   o.ProjectDir,
		hasProxy:     o.Proxy != nil,
		portBindings: o.portBindingsTCP(),
		cliName:      info.Name,
		cliVersion:   config.CLIVersion,
		gatewayTCP:   slicex.SortNumbers(config.HostGatewayTCP),
	}
}

// divergencesFrom names where the devbox's asks differ from the run's, in the run's voice
func (own devboxAsks) divergencesFrom(asked devboxAsks) []string {
	var divergences []string
	if own.projectDir != asked.projectDir {
		divergences = append(divergences, "not matching the project label")
	}
	if own.hasProxy != asked.hasProxy {
		if asked.hasProxy {
			divergences = append(divergences, "with the proxy")
		} else {
			divergences = append(divergences, "without the proxy")
		}
	}
	if !equalPortBindings(own.portBindings, asked.portBindings) {
		divergences = append(divergences, fmt.Sprintf("published %s (command asks %s)",
			portsPhrase(own.portBindings), portsPhrase(asked.portBindings)))
	}
	if !slices.Equal(own.gatewayTCP, asked.gatewayTCP) {
		divergences = append(divergences, fmt.Sprintf("opened host gateway %s (command asks %s)",
			portsCSV(own.gatewayTCP), portsCSV(asked.gatewayTCP)))
	}
	if own.cliVersion != asked.cliVersion {
		divergences = append(divergences, fmt.Sprintf("cli version %s (command asks %s)", own.cliVersion, asked.cliVersion))
	}
	if own.cliName != asked.cliName {
		divergences = append(divergences, fmt.Sprintf("installed cli %s (command asks %s)", own.cliName, asked.cliName))
	}
	return divergences
}

func portsPhrase(bindings nat.PortMap) string {
	ports := make([]string, 0, len(bindings))
	for port := range bindings {
		ports = append(ports, port.Port())
	}
	return portsCSV(ports)
}

// portsCSV renders the ports ascending — "3000, 5900"; "none" without any
func portsCSV(ports []string) string {
	if len(ports) == 0 {
		return "none"
	}
	return strings.Join(slicex.SortNumbers(ports), ", ")
}

// equalPortBindings reports two published-binding sets match — HostIP and HostPort per port
func equalPortBindings(a, b nat.PortMap) bool {
	if len(a) != len(b) {
		return false
	}
	for port, aBindings := range a {
		if !slices.Equal(aBindings, b[port]) {
			return false
		}
	}
	return true
}

// portBindingsTCP renders the run's published bindings
func (o RunOptions) portBindingsTCP() nat.PortMap {
	ports := slices.Clone(o.PortBindingsTCP)
	if o.ServeVNC {
		ports = append(ports, desktopVNCPort)
	}
	bindings := nat.PortMap{}
	for _, port := range ports {
		bindings[nat.Port(port+"/tcp")] = []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: port}}
	}
	return bindings
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
	if hostOptions.Proxy != nil {
		hostConfig.CapAdd = identity.RunCapNames()
		hostConfig.Sysctls = map[string]string{ipForwardSysctl: "0"}
	}

	hostConfig.PortBindings = hostOptions.portBindingsTCP()
	if err := refuseHeldPorts(ctxD, hostConfig.PortBindings); err != nil {
		return nil, err
	}
	if len(hostOptions.HostGatewayTCP) > 0 {
		// host.docker.internal resolves to the host's bridge address — the plain run
		// routes it direct, the egress wall's rows forward the asked ports
		hostConfig.ExtraHosts = []string{pkginfo.HostGatewayHost + ":host-gateway"}
	}
	return hostConfig, nil
}

func runConfig(o RunOptions) *container.Config {
	cfg := &container.Config{
		Image:      o.Tag,
		Labels:     map[string]string{ccboxProjectLabelKey: o.ProjectDir},
		WorkingDir: o.WorkspaceMount,
		Env:        envString(o.Env, o.Proxy != nil, len(o.HostGatewayTCP) > 0),
	}
	if o.Proxy != nil {
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
