package dmap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/ccboxtools/pkg/proxy"
	"github.com/s12chung/ccbox/ccboxtools/pkg/util/testutil"
	"github.com/s12chung/ccbox/pkg/kit/tinyproxy"
	"github.com/s12chung/ccbox/pkg/projectcfg"
)

func TestProxyMap_Options(t *testing.T) {
	testutil.Home(t)
	cfg := &projectcfg.Config{
		CLIName:   new("claude"),
		Allowlist: []string{"user.example.dev", projectcfg.DefaultsAlias, "example.com"},
	}
	allowContents := proxy.Render(cfg.AllowlistExpanded())

	proxyOptions := NewProxyMap(cfg).Options()
	assert.Equal(t, tinyproxy.Config, proxyOptions.ConfigFS)
	assert.Equal(t, map[string][]byte{pkginfo.ProxyAllowFile: allowContents}, proxyOptions.ConfigFileMap)

	require.NotNil(t, proxyOptions.OnStart)
	require.NoError(t, proxyOptions.OnStart())
	written, err := os.ReadFile(proxyAllowHostPath())
	require.NoError(t, err)
	assert.Equal(t, allowContents, written)

	require.NotNil(t, proxyOptions.OnStop)
	require.NoError(t, proxyOptions.OnStop())
	_, err = os.Stat(proxyAllowHostPath())
	require.True(t, os.IsNotExist(err))
	require.NoError(t, proxyOptions.OnStop(), "a missing file is already clean")
}

func TestProxyAllowHostDir(t *testing.T) {
	home := testutil.Home(t)
	assert.Equal(t, filepath.Join(home, ".ccbox", "tmp", "proxy"), proxyAllowHostDir())
}

func TestProxyAllowHostPath(t *testing.T) {
	home := testutil.Home(t)
	assert.Equal(t, filepath.Join(home, ".ccbox", "tmp", "proxy", "allow.txt"), proxyAllowHostPath())
}
