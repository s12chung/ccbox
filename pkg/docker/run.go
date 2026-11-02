package docker

import (
	"errors"
	"io"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"

	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/kit/seccomputil"
	"github.com/s12chung/ccbox/pkg/kit/tinyproxy"
	"github.com/s12chung/ccbox/pkg/util/klean"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/pkg/util/stream"
	"github.com/s12chung/ccbox/pkg/util/term"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/identity"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/logmux"
)

const (
	// desktopVNCPort is the port a run's desktop serves VNC on — the stock RFB port;
	// macOS Screen Sharing only takes it without the display number
	desktopVNCPort = "5900"

	// ipForwardSysctl pins the boot netns's forwarding off; the container's /proc/sys is read-only
	ipForwardSysctl = "net.ipv4.ip_forward"
)

// ExecOptions is the exec session's workload for the boot's entrypoint to run.
type ExecOptions struct {
	Cmd []string // the session's exec argv, under its wrapper
}

// Foreground streams the boot's logs
type Foreground struct {
	Writer io.Writer
	Stop   chan struct{} // made by Run: closed when the boot's log stream ends
}

// RunOptions configures the devbox container's boot and its holder session.
type RunOptions struct {
	RunHostOptions

	Exec  *ExecOptions  // the exec session's workload; nil boots without one
	Proxy *ProxyOptions // options on the boot; nil is the plain boot

	Name string // the boot's fixed container name
	Tag  string
	Env  map[string]string // container env minus the proxy vars

	HoldersDir string      // the host dir of the boot's holder flock entries
	LogFile    string      // the host file the boot's own log lines append to
	Foreground *Foreground // the run's live view of the boot's logs; nil for the exec sessions
}

func (o RunOptions) matchesContainer(labels map[string]string) bool {
	return labels[ccboxProjectLabelKey] == o.ProjectDir &&
		(labels[ccboxHasProxyLabelKey] == trueVolValue) == (o.Proxy != nil)
}

// RunHostOptions is everything mounted into the devbox
type RunHostOptions struct {
	ProjectDir     string
	WorkspaceMount string
	Mounts         []Mount
	TmpfsPaths     []string
}

func runConfig(hostOptions RunOptions) *container.Config {
	cfg := &container.Config{
		Image:      hostOptions.Tag,
		Labels:     map[string]string{ccboxProjectLabelKey: hostOptions.ProjectDir},
		WorkingDir: hostOptions.WorkspaceMount,
		Env:        envString(hostOptions.Env, hostOptions.Proxy == nil),
	}
	if hostOptions.Proxy != nil {
		// the egress wall's boot stays root to build, prove, and supervise tinyproxy —
		// workloads still arrive dropped, via the drop chain.
		//
		// Raw engine exec lands on this root too; the boot can't refuse it.
		cfg.User = "root"
		cfg.Labels[ccboxHasProxyLabelKey] = trueVolValue
	}
	return cfg
}

func runHostConfig(ctxD *dock.CtxD, hostOptions RunOptions) (*container.HostConfig, error) {
	if err := ensureMounts(ctxD, hostOptions.Tag, hostOptions.ProjectDir, hostOptions.Mounts); err != nil {
		return nil, err
	}
	hostConfig := &container.HostConfig{
		CapDrop:     []string{"ALL"},
		SecurityOpt: []string{"no-new-privileges"},
		// Loopback-only: native RFB is VncAuth-password auth on a
		// plain wire, so keep it off the network.
		PortBindings: nat.PortMap{desktopVNCPort + "/tcp": {{HostIP: "127.0.0.1", HostPort: desktopVNCPort}}},
		Binds:        mountSpecs(hostOptions.Mounts),
		Tmpfs:        tmpfsMap(hostOptions.TmpfsPaths),
	}
	seccompOpt, err := seccomputil.BootOpt()
	if err != nil {
		return nil, err
	}
	hostConfig.SecurityOpt = append(hostConfig.SecurityOpt, seccompOpt)
	if hostOptions.Proxy != nil {
		hostConfig.CapAdd = identity.RunCapNames()
		hostConfig.Sysctls = map[string]string{ipForwardSysctl: "0"}
	}
	return hostConfig, nil
}

func (o RunOptions) streamOptions() (stream.Options, error) {
	bootFile, err := osutil.OpenAppend(o.LogFile)
	if err != nil {
		return stream.Options{}, err
	}
	var clean klean.Joiner
	clean.Push("close the boot's log file", bootFile.Close)

	opts := stream.Options{Writers: []io.Writer{bootFile}, OnEnd: clean.Run}

	if o.Proxy != nil {
		proxyFile, err := osutil.OpenAppend(o.Proxy.LogFile)
		if err != nil {
			return stream.Options{}, errors.Join(err, clean.Run())
		}
		clean.Push("close the proxy's log file", proxyFile.Close)
		demuxer := &logmux.Demuxer{Writers: map[string]io.Writer{
			"":                   bootFile,
			pkginfo.ProxyMuxName: proxyFile,
		}}
		clean.Push("file the stream's torn tail", demuxer.Flush)

		opts.Writers = []io.Writer{demuxer}
		opts.ReadyWait = &stream.ReadyWait{Marker: pkginfo.ReadyMarker}
		if o.Foreground == nil {
			opts.ReadyWait.Screen = term.NewColorWriter(os.Stdout, tinyproxy.LevelColor)
		}
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
