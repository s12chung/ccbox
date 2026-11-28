package docker

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/kit/dock"
	"github.com/s12chung/ccbox/pkg/util/must"
	"github.com/s12chung/ccbox/pkg/util/stream"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/identity"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/logmux"
)

func TestRunOptions_ConflictErr(t *testing.T) {
	_, unreadable := pkginfo.RunConfigFromJSON("{") // the devbox's own parse failure, named back to the user
	require.Error(t, unreadable)
	tests := []struct {
		name string
		o    RunOptions
		c    *container.InspectResponse
		want string // empty: the devbox answers the run's asks
	}{
		{
			name: "Plain",
			o:    plainRun(),
			c:    labeledRun(false),
		},
		{
			name: "Proxy",
			o:    proxyRun(),
			c:    labeledRun(true),
		},
		{
			name: "UnlabeledPreLabel",
			o:    plainRun(),
			c:    inspectRun(nil),
			want: conflict("not matching the project label"),
		},
		{
			name: "UnlabeledPreLabel_Proxy",
			o:    proxyRun(),
			c:    inspectRun(nil),
			want: conflict("not matching the project label and with the proxy"),
		},
		{
			name: "DifferingProxy",
			o:    plainRun(),
			c:    labeledRun(true),
			want: conflict("without the proxy"),
		},
		{
			name: "UnreadableEnv",
			o:    plainRun(),
			c:    labeledRun(false, pkginfo.RunConfigEnvVar+"={"),
			want: "devbox env: " + unreadable.Error(),
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

func TestRunOptions_ConflictErr_EnvAsks(t *testing.T) {
	tests := []struct {
		name string
		o    RunOptions
		c    *container.InspectResponse
		want string // empty: the devbox answers the run's asks
	}{
		{
			name: "Ports",
			o:    withPorts(plainRun(), []string{"3000", "7200"}),
			c:    labeledRun(false, pkginfo.RunConfigEnvVar+"="+boundBody([]string{"3000", "7200"})),
		},
		{
			name: "DifferingPorts",
			o:    withPorts(plainRun(), []string{"3000"}),
			c:    labeledRun(false),
			want: conflict("published none (command asks 3000)"),
		},
		{
			name: "SwappedPorts",
			o:    withPorts(plainRun(), []string{"3000"}),
			c:    labeledRun(false, pkginfo.RunConfigEnvVar+"="+boundBody([]string{"9700", "12345"})),
			want: conflict("published 9700, 12345 (command asks 3000)"),
		},
		{
			name: "HostGateway",
			o:    withHostGateway(plainRun(), []string{"5432"}),
			c:    labeledRun(false, pkginfo.RunConfigEnvVar+"="+runConfigBody(pkginfo.LatestVersion, []string{"5432"})),
		},
		{
			name: "HostGatewayOrderless",
			o:    withHostGateway(plainRun(), []string{"5432", "6379"}),
			c:    labeledRun(false, pkginfo.RunConfigEnvVar+"="+runConfigBody(pkginfo.LatestVersion, []string{"6379", "5432"})),
		},
		{
			name: "DifferingHostGateway",
			o:    withHostGateway(plainRun(), []string{"3306"}),
			c:    labeledRun(false, pkginfo.RunConfigEnvVar+"="+runConfigBody(pkginfo.LatestVersion, []string{"5432", "6379"})),
			want: conflict("opened host gateway 5432, 6379 (command asks 3306)"),
		},
		{
			name: "CLI",
			o:    withCLIEnv(plainRun(), "pi", "0.9.2"),
			c:    labeledRun(false, pkginfo.EnvVar+"="+cliBody("pi"), pkginfo.RunConfigEnvVar+"="+runConfigBody("0.9.2", nil)),
		},
		{
			name: "DifferingCLIVersion",
			o:    withCLIEnv(plainRun(), "pi", "0.9.2"),
			c:    labeledRun(false, pkginfo.EnvVar+"="+cliBody("pi"), pkginfo.RunConfigEnvVar+"="+runConfigBody(pkginfo.LatestVersion, nil)),
			want: conflict("cli version latest (command asks 0.9.2)"),
		},
		{
			name: "DifferingCLIName",
			o:    withCLIEnv(plainRun(), "claude", pkginfo.LatestVersion),
			c:    labeledRun(false, pkginfo.EnvVar+"="+cliBody("pi"), pkginfo.RunConfigEnvVar+"="+runConfigBody(pkginfo.LatestVersion, nil)),
			want: conflict("installed cli pi (command asks claude)"),
		},
		{
			name: "GUIApp",
			o:    withGUIAppEnv(plainRun(), "ide", new("1.2.3")),
			c:    labeledRun(false, guiAppEnv("ide", new("1.2.3"))...),
		},
		{
			name: "DifferingGUIAppVersion",
			o:    withGUIAppEnv(plainRun(), "ide", new("1.2.3")),
			c:    labeledRun(false, guiAppEnv("ide", new(pkginfo.LatestVersion))...),
			want: conflict("gui app version latest (command asks 1.2.3)"),
		},
		{
			name: "DifferingGUIAppName",
			o:    withGUIAppEnv(plainRun(), "other", new(pkginfo.LatestVersion)),
			c:    labeledRun(false, guiAppEnv("ide", new(pkginfo.LatestVersion))...),
			want: conflict("installed gui app ide (command asks other)"),
		},
		{
			name: "GUIAppAgainstNone",
			o:    withGUIAppEnv(plainRun(), "ide", new(pkginfo.LatestVersion)),
			c:    labeledRun(false, pkginfo.VNCConfigEnvVar+"={}"),
			want: conflict("gui app version none (command asks latest) and installed gui app none (command asks ide)"),
		},
		{
			name: "NoneAgainstGUIApp",
			o:    plainRun(),
			c:    labeledRun(false, guiAppEnv("ide", new(pkginfo.LatestVersion))...),
			want: conflict("gui app version latest (command asks none) and installed gui app ide (command asks none)"),
		},
		{
			name: "ProxyAndPorts",
			o:    withPorts(proxyRun(), []string{"3000"}),
			c:    labeledRun(false),
			want: conflict("with the proxy and published none (command asks 3000)"),
		},
		{
			name: "ProxyAndHostGateway",
			o:    withHostGateway(proxyRun(), []string{"3306"}),
			c:    labeledRun(false, pkginfo.RunConfigEnvVar+"="+runConfigBody(pkginfo.LatestVersion, nil)),
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

// inspectRun fakes the devbox's inspect: its labels and env — a devbox booted by
// this ccbox's env unless the case says otherwise
func inspectRun(labels map[string]string, env ...string) *container.InspectResponse {
	return &container.InspectResponse{
		Config: &container.Config{
			Labels: labels,
			Env:    slices.Concat(env, []string{pkginfo.EnvVar + "=" + freshPkgInfo, pkginfo.RunConfigEnvVar + "=" + freshRunConfig}),
		},
	}
}

func labeledRun(hasProxy bool, env ...string) *container.InspectResponse {
	labels := map[string]string{ccboxProjectLabelKey: "/host/proj"}
	if hasProxy {
		labels[ccboxHasProxyLabelKey] = trueVolValue
	}
	return inspectRun(labels, env...)
}

func conflict(divergences string) string {
	return "devbox ccbox-proj is already running for this project, " + divergences + " — exit its session or `docker rm -f ccbox-proj`"
}

// withPorts aims o's run at the given published ports, in its config too
func withPorts(o RunOptions, ports []string) RunOptions {
	o.PortBindingsTCP = ports
	return withRunConfig(o, boundBody(ports))
}

// withHostGateway aims o's run at the given host-gateway ports, in its config too
func withHostGateway(o RunOptions, ports []string) RunOptions {
	o.HostGatewayTCP = ports
	return withRunConfig(o, runConfigBody(pkginfo.LatestVersion, ports))
}

// withCLIEnv aims o's run at the given cli and version, in its config too
func withCLIEnv(o RunOptions, name, version string) RunOptions {
	o = withRunConfig(o, runConfigBody(version, nil))
	o.Env[pkginfo.EnvVar] = cliBody(name)
	return o
}

// withGUIAppEnv aims o's run at the given gui app and version, in its config too —
// a nil version names none
func withGUIAppEnv(o RunOptions, name string, version *string) RunOptions {
	for _, entry := range guiAppEnv(name, version) {
		key, value, _ := strings.Cut(entry, "=")
		o.Env[key] = value
	}
	return o
}

// cliBody renders a CLI_PKGINFO env body in the host's shape
func cliBody(name string) string {
	return body(pkginfo.PkgInfo{Name: name, Npm: &pkginfo.Npm{Package: "a"}})
}

// vncBody renders a VNC_CONFIG env body in the host's shape — "{}" names no gui app
func vncBody(name string) string {
	var info pkginfo.VNCInfo
	if name != "" {
		info.GUIApp = &pkginfo.GUIPkgInfo{PkgInfo: pkginfo.PkgInfo{Name: name, Npm: &pkginfo.Npm{Package: "a"}}}
	}
	return body(info)
}

// guiAppEnv is the env entries naming the gui app at version over the latest cli —
// a nil version names none
func guiAppEnv(name string, version *string) []string {
	return []string{pkginfo.VNCConfigEnvVar + "=" + vncBody(name), pkginfo.RunConfigEnvVar + "=" + guiAppRunConfigBody(version)}
}

// guiAppRunConfigBody renders a CCBOX_CONFIG env body over the latest cli, naming
// the gui app's version
func guiAppRunConfigBody(version *string) string {
	return body(pkginfo.RunConfig{CLIVersion: pkginfo.LatestVersion, GUIAppVersion: version})
}

// body renders a pkginfo env body in the host's shape
func body[T any](v T) string { return string(must.Get(json.Marshal(v))) }

// the env a devbox booted by this ccbox carries: a latest pi, no ports
var (
	freshPkgInfo   = cliBody("pi")
	freshRunConfig = runConfigBody(pkginfo.LatestVersion, nil)
)

// runConfigBody renders a CCBOX_CONFIG env body in the host's shape
func runConfigBody(version string, gateway []string) string {
	return body(pkginfo.RunConfig{CLIVersion: version, HostGatewayTCP: gateway})
}

// boundBody renders a CCBOX_CONFIG env body over the latest cli, binding the ports
func boundBody(ports []string) string {
	return body(pkginfo.RunConfig{CLIVersion: pkginfo.LatestVersion, PortBindingsTCP: ports})
}

// withRunConfig aims o's run at a CCBOX_CONFIG env body
func withRunConfig(o RunOptions, body string) RunOptions {
	o.Env[pkginfo.RunConfigEnvVar] = body
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
			// /dev/null over a workspace file: a file mask, read-write like any bind
			Mounts:     []Mount{NewBind("/host/proj", "/home/ccbox/proj"), NewBind("/dev/null", "/home/ccbox/proj/notes.txt")},
			TmpfsPaths: []string{"/home/ccbox/.cache"},
		},
	}

	hostConfig, err := runHostConfig(nil, opts)

	require.NoError(t, err)
	assert.Empty(t, hostConfig.NetworkMode, "the devbox uses the engine's default bridge")
	assert.Equal(t, []string{"ALL"}, []string(hostConfig.CapDrop))
	assert.Empty(t, hostConfig.CapAdd, "the plain run arrives as the image's user, capless")
	assert.Equal(t, []string{"/host/proj:/home/ccbox/proj", "/dev/null:/home/ccbox/proj/notes.txt"}, hostConfig.Binds)
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
		Env:            map[string]string{pkginfo.EnvVar: freshPkgInfo, pkginfo.RunConfigEnvVar: freshRunConfig},
	}
}

func proxyRun() RunOptions {
	o := plainRun()
	o.Proxy = &ProxyOptions{}
	return o
}
