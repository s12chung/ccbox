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

func TestRunOptions_MatchesContainer(t *testing.T) {
	tests := []struct {
		name    string
		o       RunOptions
		cLabels map[string]string
		matches bool
	}{
		{
			name:    "Plain",
			o:       plainRun(),
			cLabels: map[string]string{ccboxProjectLabelKey: "/host/proj"},
			matches: true,
		},
		{
			name: "Proxy",
			o:    proxyRun(),
			cLabels: map[string]string{
				ccboxProjectLabelKey:  "/host/proj",
				ccboxHasProxyLabelKey: trueVolValue,
			},
			matches: true,
		},
		{
			name:    "DifferingProxy",
			o:       plainRun(),
			cLabels: map[string]string{ccboxProjectLabelKey: "/host/proj", ccboxHasProxyLabelKey: trueVolValue},
		},
		{
			name: "UnlabeledPreLabel",
			o:    plainRun(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.matches, tt.o.matchesContainer(tt.cLabels))
		})
	}
}

func TestRunOptions_StreamOptions(t *testing.T) {
	var foreground bytes.Buffer
	withForeground := func(o RunOptions) RunOptions {
		o.Foreground = &Foreground{Writer: &foreground, Stop: make(chan struct{})}
		return o
	}
	tests := []struct {
		name       string
		o          RunOptions
		wantMarker string
	}{
		{name: "PlainHolders", o: withForeground(plainRun()), wantMarker: pkginfo.DevboxReadyMarker},
		{name: "WalledHolders", o: withForeground(proxyRun()), wantMarker: pkginfo.ReadyMarker},
		{name: "WalledSession", o: proxyRun(), wantMarker: pkginfo.ReadyMarker},
		{name: "PlainSession", o: plainRun(), wantMarker: pkginfo.DevboxReadyMarker},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, proxyLog := withLogFiles(t, tt.o)
			opts, err := o.streamOptions()
			require.NoError(t, err)
			assertWait(t, tt.wantMarker, o, opts.ReadyWait)
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
func assertWait(t *testing.T, wantMarker string, o RunOptions, wait *stream.ReadyWait) {
	t.Helper()
	if wantMarker == "" {
		assert.Nil(t, wait)
		return
	}
	require.NotNil(t, wait)
	assert.Equal(t, wantMarker, wait.Marker)
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
	// hermetic to this machine's port state: the run assumes the VNC port free
	refuseHeldPorts = func(*dock.CtxD, nat.PortMap) error { return nil }
	t.Cleanup(func() { refuseHeldPorts = dock.RefuseHeldPorts })

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
	assert.Equal(t, nat.PortMap{
		desktopVNCPort + "/tcp": {{HostIP: "127.0.0.1", HostPort: desktopVNCPort}},
	}, hostConfig.PortBindings, "the unwalled boot may serve the desktop")
	require.Len(t, hostConfig.SecurityOpt, 2)
	assert.Equal(t, "no-new-privileges", hostConfig.SecurityOpt[0])
	assert.True(t, strings.HasPrefix(hostConfig.SecurityOpt[1], "seccomp="))
	assert.Empty(t, hostConfig.Sysctls)
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
	assert.Empty(t, hostConfig.PortBindings, "the proxied boot never serves the desktop: it claims no host port")
	require.Len(t, hostConfig.SecurityOpt, 2, "every devbox mounts the seccomp profile")
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
