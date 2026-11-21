package docker

import (
	"bytes"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/util/stream"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/identity"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/logmux"
)

func TestRunOptions_ConflictErr(t *testing.T) {
	loopback := func(port string) []nat.PortBinding {
		return []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: port}}
	}
	labeledRun := func(hasProxy bool, bindings nat.PortMap, env ...string) *container.InspectResponse {
		labels := map[string]string{ccboxProjectLabelKey: "/host/proj"}
		if hasProxy {
			labels[ccboxHasProxyLabelKey] = trueVolValue
		}
		return inspectRun(labels, bindings, env...)
	}
	conflict := func(divergences string) string {
		return "devbox ccbox-proj is already running for this project, " + divergences + " — exit its session or `docker rm -f ccbox-proj`"
	}
	tests := []struct {
		name string
		o    RunOptions
		c    *container.InspectResponse
		want string // empty: the devbox answers the run's asks
	}{
		{
			name: "Plain",
			o:    plainRun(),
			c:    labeledRun(false, nil),
		},
		{
			name: "Proxy",
			o:    proxyRun(),
			c:    labeledRun(true, nil),
		},
		{
			name: "Ports",
			o:    withPorts(plainRun(), []string{"3000", "7200"}),
			c: labeledRun(false, nat.PortMap{
				"3000/tcp": loopback("3000"),
				"7200/tcp": loopback("7200"),
			}),
		},
		{
			name: "HostGateway",
			o:    withHostGateway(plainRun(), []string{"5432"}),
			c: labeledRun(false, nil,
				pkginfo.HostGatewayTCPEnvVar+"=5432"),
		},
		{
			name: "HostGatewayOrderless",
			o:    withHostGateway(plainRun(), []string{"5432", "6379"}),
			c: labeledRun(false, nil,
				pkginfo.HostGatewayTCPEnvVar+"=6379,5432"),
		},
		{
			name: "UnlabeledPreLabel",
			o:    plainRun(),
			c:    inspectRun(nil, nil),
			want: conflict("not matching the project label"),
		},
		{
			name: "UnlabeledPreLabel_Proxy",
			o:    proxyRun(),
			c:    inspectRun(nil, nil),
			want: conflict("not matching the project label and with the proxy"),
		},
		{
			name: "DifferingProxy",
			o:    plainRun(),
			c:    labeledRun(true, nil),
			want: conflict("without the proxy"),
		},
		{
			name: "DifferingPorts",
			o:    withPorts(plainRun(), []string{"3000"}),
			c:    labeledRun(false, nil),
			want: conflict("published none (command asks 3000)"),
		},
		{
			name: "DifferingBinding",
			o:    withPorts(plainRun(), []string{"3000"}),
			c:    labeledRun(false, nat.PortMap{"3000/tcp": []nat.PortBinding{{HostPort: "3000"}}}),
			want: conflict("published 3000 (command asks 3000)"),
		},
		{
			name: "ProxyAndPorts",
			o:    withPorts(proxyRun(), []string{"3000"}),
			c:    labeledRun(false, nil),
			want: conflict("with the proxy and published none (command asks 3000)"),
		},
		{
			name: "SwappedPorts",
			o:    withPorts(plainRun(), []string{"3000"}),
			c: labeledRun(false, nat.PortMap{
				"12345/tcp": {{HostIP: "127.0.0.1", HostPort: "12345"}},
				"9700/tcp":  {{HostIP: "127.0.0.1", HostPort: "9700"}},
			}),
			want: conflict("published 9700, 12345 (command asks 3000)"),
		},
		{
			name: "DifferingHostGateway",
			o:    withHostGateway(plainRun(), []string{"3306"}),
			c: labeledRun(false, nil,
				pkginfo.HostGatewayTCPEnvVar+"=5432,6379"),
			want: conflict("opened host gateway 5432, 6379 (command asks 3306)"),
		},
		{
			name: "ProxyAndHostGateway",
			o:    withHostGateway(proxyRun(), []string{"3306"}),
			c:    labeledRun(false, nil),
			want: conflict("with the proxy and opened host gateway none (command asks 3306)"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.o.conflictErr(tt.c)
			if tt.want == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.want)
			}
		})
	}
}

// inspectRun fakes the devbox's inspect: its labels, published bindings, and env
func inspectRun(labels map[string]string, bindings nat.PortMap, env ...string) *container.InspectResponse {
	return &container.InspectResponse{
		ContainerJSONBase: &container.ContainerJSONBase{HostConfig: &container.HostConfig{PortBindings: bindings}},
		Config:            &container.Config{Labels: labels, Env: env},
	}
}

// withPorts aims o's run at the given published ports
func withPorts(o RunOptions, ports []string) RunOptions {
	o.PortBindingsTCP = ports
	return o
}

// withHostGateway aims o's run at the given host-gateway ports
func withHostGateway(o RunOptions, ports []string) RunOptions {
	o.HostGatewayTCP = ports
	return o
}

func TestRunOptions_StreamOptions(t *testing.T) {
	var foreground bytes.Buffer
	withForeground := func(o RunOptions) RunOptions {
		o.Foreground = &Foreground{Writer: &foreground, Stop: make(chan struct{})}
		return o
	}
	tests := []struct {
		name string
		o    RunOptions
	}{
		{name: "PlainHolders", o: withForeground(plainRun())},
		{name: "WalledHolders", o: withForeground(proxyRun())},
		{name: "WalledSession", o: proxyRun()},
		{name: "PlainSession", o: plainRun()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, proxyLog := withLogFiles(t, tt.o)
			opts, err := o.streamOptions()
			require.NoError(t, err)
			assertWait(t, o, opts.ReadyWait)
			assertWriters(t, &foreground, o, opts.Writers)
			assertFiles(t, o, proxyLog, opts)
		})
	}
}

// withLogFiles aims o's log files at a test dir, yielding the proxy's path —
// set on o's options only with the egress wall, so a plain run's read finds no file
func withLogFiles(t *testing.T, o RunOptions) (RunOptions, string) {
	t.Helper()
	dir := t.TempDir()
	o.LogFile = filepath.Join(dir, "devbox.log")
	proxyLog := filepath.Join(dir, "proxy.log")
	if o.Proxy != nil {
		o.Proxy.LogFile = proxyLog
	}
	return o, proxyLog
}

// assertWait checks the ready wait: every run waits its boot's marker and
// its screen is only for the session holding no foreground view
func assertWait(t *testing.T, o RunOptions, wait *stream.ReadyWait) {
	t.Helper()
	require.NotNil(t, wait)
	assert.Equal(t, pkginfo.ReadyMarker, wait.Marker)
	if o.Foreground != nil {
		assert.Nil(t, wait.Screen, "the holders' foreground view needs no wait screen")
	} else {
		assert.NotNil(t, wait.Screen, "the session's wait view colors stdout")
	}
}

// assertWriters checks the stream's sinks: the file sink first, the holders'
// foreground view appended
func assertWriters(t *testing.T, fg io.Writer, o RunOptions, writers []io.Writer) {
	t.Helper()
	if o.Foreground != nil {
		require.Len(t, writers, 2, "the file sink and the holders' foreground view")
		assert.Same(t, fg, writers[1])
		return
	}
	assert.Len(t, writers, 1, "the plain session streams the file sink alone")
}

// assertFiles writes a boot line — and with the egress wall, a muxed tinyproxy line —
// through the stream's sink, then checks the OnEnd'd files: the boot's lines
// file whole, tinyproxy's tag-stripped, no proxy file without the egress wall
func assertFiles(t *testing.T, o RunOptions, proxyLog string, opts stream.Options) {
	t.Helper()
	_, err := opts.Writers[0].Write([]byte("boot: installing\n"))
	require.NoError(t, err)
	if o.Proxy != nil {
		muxed := &bytes.Buffer{}
		_, err = logmux.NewMuxer(pkginfo.ProxyMuxName, muxed).Write([]byte("INFO listening\n"))
		require.NoError(t, err)
		_, err = opts.Writers[0].Write(muxed.Bytes())
		require.NoError(t, err)
	}
	require.NoError(t, opts.OnEnd(), "the stream's cleanup runs clean")

	bootBody, err := os.ReadFile(o.LogFile) // #nosec G304 -- the test's own log file
	require.NoError(t, err)
	assert.Equal(t, "boot: installing\n", string(bootBody))
	proxyBody, err := os.ReadFile(proxyLog) // #nosec G304 -- the test's own log file
	if o.Proxy != nil {
		require.NoError(t, err)
		assert.Equal(t, "INFO listening\n", string(proxyBody), "tinyproxy's lines file without their tag")
	} else {
		assert.ErrorIs(t, err, fs.ErrNotExist, "no proxy file without the egress wall")
	}
}

func TestTakeForegroundMark_Held(t *testing.T) {
	o := RunOptions{
		Name:       "ccbox-proj",
		HoldersDir: t.TempDir(),
		LogFile:    "/home/me/.ccbox/logs/proj/devbox.log",
	}
	release, err := takeForegroundMark(o)
	require.NoError(t, err)
	defer func() { require.NoError(t, release()) }()

	_, err = takeForegroundMark(o)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ccbox-proj is already in the foreground")
}

func TestTakeForegroundMark_Release(t *testing.T) {
	o := RunOptions{HoldersDir: t.TempDir()}
	release, err := takeForegroundMark(o)
	require.NoError(t, err)
	require.NoError(t, release())

	foreground, err := isForegrounded(foregroundMarkPath(o.HoldersDir))
	require.NoError(t, err)
	assert.False(t, foreground, "a released mark foregrounds no one")
}

func TestForegroundMarkPath(t *testing.T) {
	assert.Equal(t, filepath.Join("holders", "foreground"), foregroundMarkPath("holders"),
		"the mark sits in the holders dir, swept around by its holder entries")
}

func TestRunConfig(t *testing.T) {
	tests := []struct {
		name string
		opts RunOptions
		want *container.Config
	}{
		{
			name: "Base",
			opts: RunOptions{
				RunHostOptions: RunHostOptions{
					ProjectDir:     "/host/proj",
					WorkspaceMount: "/home/ccbox/proj",
				},
				Tag: "ccbox:latest",
				Env: map[string]string{"GH_TOKEN": "gh", "GOFLAGS": "-mod=mod"},
			},
			want: &container.Config{
				Image:      "ccbox:latest",
				Labels:     map[string]string{"ccbox.project": "/host/proj"},
				WorkingDir: "/home/ccbox/proj",
				Env: []string{
					"GH_TOKEN=gh",
					"GOFLAGS=-mod=mod",
				},
			},
		},
		{
			name: "EgressWall",
			opts: RunOptions{
				RunHostOptions: RunHostOptions{
					ProjectDir:     "/host/proj",
					WorkspaceMount: "/home/ccbox/proj",
				},
				Proxy: &ProxyOptions{},
				Tag:   "ccbox:latest",
				Env:   map[string]string{"GH_TOKEN": "gh"},
			},
			want: &container.Config{
				Image: "ccbox:latest",
				User:  "root",
				Labels: map[string]string{
					"ccbox.project":       "/host/proj",
					ccboxHasProxyLabelKey: trueVolValue,
				},
				WorkingDir: "/home/ccbox/proj",
				Env: []string{
					"GH_TOKEN=gh",
					// last: they override user settings, aimed at the fixed parent veth IP
					"http_proxy=http://" + pkginfo.ProxyAddr + ":" + strconv.Itoa(pkginfo.ProxyPort),
					"https_proxy=http://" + pkginfo.ProxyAddr + ":" + strconv.Itoa(pkginfo.ProxyPort),
					"no_proxy=localhost,127.0.0.1,::1",
					"NO_PROXY=localhost,127.0.0.1,::1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, runConfig(tt.opts))
		})
	}
}

func TestRunHostConfig(t *testing.T) {
	opts := RunOptions{
		RunHostOptions: RunHostOptions{
			ProjectDir: "/host/proj",
			Mounts:     []Mount{NewBind("/host/proj", "/home/ccbox/proj")},
			TmpfsPaths: []string{"/home/ccbox/.cache"},
		},
	}

	hostConfig, err := runHostConfig(nil, opts)

	require.NoError(t, err)
	assert.Empty(t, hostConfig.NetworkMode, "the devbox uses the engine's default bridge")
	assert.Equal(t, []string{"ALL"}, []string(hostConfig.CapDrop))
	assert.Empty(t, hostConfig.CapAdd, "the plain run arrives as the image's user, capless")
	assert.Equal(t, []string{"/host/proj:/home/ccbox/proj"}, hostConfig.Binds)
	assert.Equal(t, map[string]string{"/home/ccbox/.cache": tmpfsOpts}, hostConfig.Tmpfs)
	assert.Empty(t, hostConfig.PortBindings, "the headless run serves no desktop: it claims no host port")
	require.Len(t, hostConfig.SecurityOpt, 2)
	assert.Equal(t, "no-new-privileges", hostConfig.SecurityOpt[0])
	assert.True(t, strings.HasPrefix(hostConfig.SecurityOpt[1], "seccomp="))
	assert.Empty(t, hostConfig.Sysctls)
}

func TestRunHostConfig_VNC(t *testing.T) {
	// hermetic to this machine's port state: the run assumes the VNC port free
	refuseHeldPorts = func(*dock.CtxD, nat.PortMap) error { return nil }
	t.Cleanup(func() { refuseHeldPorts = dock.RefuseHeldPorts })

	opts := RunOptions{ServeVNC: true}

	hostConfig, err := runHostConfig(nil, opts)

	require.NoError(t, err)
	assert.Equal(t, nat.PortMap{
		desktopVNCPort + "/tcp": {{HostIP: "127.0.0.1", HostPort: desktopVNCPort}},
	}, hostConfig.PortBindings, "the desktop serves the host's loopback, with or without the egress wall")
}

func TestRunHostConfig_EgressWall(t *testing.T) {
	opts := RunOptions{
		Proxy: &ProxyOptions{},
	}

	hostConfig, err := runHostConfig(nil, opts)

	require.NoError(t, err)
	// the proxied devbox's run budget, exact-matched by the entrypoint's verify
	assert.Equal(t, identity.RunCapNames(), []string(hostConfig.CapAdd))
	assert.Equal(t, []string{"ALL"}, []string(hostConfig.CapDrop))
	assert.Equal(t, map[string]string{ipForwardSysctl: "0"}, hostConfig.Sysctls)
	assert.Empty(t, hostConfig.PortBindings, "the proxied headless run serves no desktop: it claims no host port")
	require.Len(t, hostConfig.SecurityOpt, 2, "every devbox mounts the seccomp profile")
}

func TestRunHostConfig_EgressWall_VNC(t *testing.T) {
	// hermetic to this machine's port state: the run assumes the VNC port free
	refuseHeldPorts = func(*dock.CtxD, nat.PortMap) error { return nil }
	t.Cleanup(func() { refuseHeldPorts = dock.RefuseHeldPorts })

	opts := RunOptions{
		Proxy:    &ProxyOptions{},
		ServeVNC: true,
	}

	hostConfig, err := runHostConfig(nil, opts)

	require.NoError(t, err)
	assert.Equal(t, identity.RunCapNames(), []string(hostConfig.CapAdd), "the egress wall's budget stands beside the desktop's port")
	assert.Equal(t, map[string]string{ipForwardSysctl: "0"}, hostConfig.Sysctls)
	assert.Equal(t, nat.PortMap{
		desktopVNCPort + "/tcp": {{HostIP: "127.0.0.1", HostPort: desktopVNCPort}},
	}, hostConfig.PortBindings, "the egress wall seals egress, not the published port: the desktop serves the host's loopback")
}

func TestRunHostConfig_PortBindings(t *testing.T) {
	// hermetic to this machine's port state: the run assumes published ports free
	refuseHeldPorts = func(*dock.CtxD, nat.PortMap) error { return nil }
	t.Cleanup(func() { refuseHeldPorts = dock.RefuseHeldPorts })

	loopback := func(port string) []nat.PortBinding {
		return []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: port}}
	}
	tests := []struct {
		name string
		o    RunOptions
		want nat.PortMap
	}{
		{"headless and proxied variants claim none", RunOptions{Proxy: &ProxyOptions{}}, nat.PortMap{}},
		{
			"the user's ports alone",
			RunOptions{PortBindingsTCP: []string{"3000", "7200"}},
			nat.PortMap{"3000/tcp": loopback("3000"), "7200/tcp": loopback("7200")},
		},
		{
			"the desktop's binding stands beside the user's ports",
			RunOptions{ServeVNC: true, PortBindingsTCP: []string{"3000"}},
			nat.PortMap{desktopVNCPort + "/tcp": loopback(desktopVNCPort), "3000/tcp": loopback("3000")},
		},
		{
			"a 5900 publish collapses into the desktop's identical binding",
			RunOptions{ServeVNC: true, PortBindingsTCP: []string{desktopVNCPort}},
			nat.PortMap{desktopVNCPort + "/tcp": loopback(desktopVNCPort)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hostConfig, err := runHostConfig(nil, tt.o)
			require.NoError(t, err)
			assert.Equal(t, tt.want, hostConfig.PortBindings)
		})
	}
}

func TestRunHostConfig_HostGateway(t *testing.T) {
	entry := []string{pkginfo.HostGatewayHost + ":host-gateway"}
	tests := []struct {
		name string
		o    RunOptions
		want []string
	}{
		{"no ask, no entry", RunOptions{}, nil},
		{"the ask writes the hosts entry on a plain run", RunOptions{HostGatewayTCP: []string{"5432"}}, entry},
		{"and on a proxied one", RunOptions{Proxy: &ProxyOptions{}, HostGatewayTCP: []string{"5432", "6379"}}, entry},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hostConfig, err := runHostConfig(nil, tt.o)
			require.NoError(t, err)
			assert.Equal(t, tt.want, hostConfig.ExtraHosts)
		})
	}
}

func plainRun() RunOptions {
	return RunOptions{
		RunHostOptions: RunHostOptions{ProjectDir: "/host/proj"},
		Name:           "ccbox-proj",
	}
}

func proxyRun() RunOptions {
	o := plainRun()
	o.Proxy = &ProxyOptions{}
	return o
}
