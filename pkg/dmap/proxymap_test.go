package dmap

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/kit/tinyproxy"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/proxy"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestProxyMap_Options(t *testing.T) {
	testutil.Home(t)
	cfg := &projectcfg.Config{
		CLIName:   new("claude"),
		Allowlist: []string{"user.example.dev", projectcfg.DefaultsAlias, "example.com"},
	}
	allowContents := proxy.Render(cfg.AllowlistExpanded())
	conf := tinyproxy.MustConf()

	options := NewProxyMap(cfg).Options(nil)
	assert.Equal(t, proxyLiveDir(), options.HostDir)
	require.True(t, ioutil.Missing(proxyLiveDir()), "rendering alone writes nothing")

	require.NotNil(t, options.BeforeStart)
	require.NoError(t, options.BeforeStart())
	written, err := os.ReadFile(proxyAllowPath())
	require.NoError(t, err)
	assert.Equal(t, allowContents, written)
	written, err = os.ReadFile(filepath.Join(proxyLiveDir(), tinyproxy.ConfFile))
	require.NoError(t, err)
	assert.Equal(t, conf, written)
	// a racing reload never reads a partial file: the dir holds exactly the two configs,
	// with no temp file left behind
	entries, err := os.ReadDir(proxyLiveDir())
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	assert.ElementsMatch(t, []string{tinyproxy.ConfFile, pkginfo.ProxyAllowFile}, names)

	require.NotNil(t, options.OnStop)
	require.NoError(t, options.OnStop())
	require.True(t, ioutil.Present(proxyLiveDir()), "the dir stays: a devbox may still bind it")
	entries, err = os.ReadDir(proxyLiveDir())
	require.NoError(t, err)
	assert.Empty(t, entries)
	require.NoError(t, options.OnStop(), "an empty dir is already clean")
}

func TestProxyLiveDir(t *testing.T) {
	home := testutil.Home(t)
	assert.Equal(t, filepath.Join(home, ".ccbox", "tmp", "proxy"), proxyLiveDir())
}

func TestProxyAllowPath(t *testing.T) {
	home := testutil.Home(t)
	assert.Equal(t, filepath.Join(home, ".ccbox", "tmp", "proxy", "allow.txt"), proxyAllowPath())
}
