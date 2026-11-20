package dmap

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/s12chung/ccbox/pkg/cfg"
	"github.com/s12chung/ccbox/pkg/util/osutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/security/allowfile"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestProxyMap_Options(t *testing.T) {
	testutil.Home(t)
	config := &cfg.Config{
		CLIName: "claude",
		Proxy:   &cfg.Proxy{Allowlist: []string{"user.example.dev", cfg.DefaultsAlias, "example.com"}},
	}
	allowContents := allowfile.Render(config.AllowlistExpanded())

	var reported []string
	options := NewProxyMap(config).Options(func(changed string) { reported = append(reported, changed) })
	assert.Equal(t, proxyLiveDir(), options.HostDir)
	assert.Equal(t, proxyLogPath(config.ProjectDir(), false), options.LogFile)
	require.True(t, osutil.Missing(proxyLiveDir()), "rendering alone writes nothing")
	assert.Empty(t, reported, "rendering alone reports nothing")

	require.NotNil(t, options.BeforeStart)
	require.NoError(t, options.BeforeStart())
	written, err := os.ReadFile(proxyAllowPath())
	require.NoError(t, err)
	assert.Equal(t, allowContents, written)
	// a racing reload never reads a partial file: the dir holds exactly the allow file,
	// with no temp file left behind
	entries, err := os.ReadDir(proxyLiveDir())
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	assert.Equal(t, []string{pkginfo.ProxyAllowFile}, names)

	require.NotNil(t, options.OnStop)
	require.NoError(t, options.OnStop())
	require.True(t, osutil.Present(proxyLiveDir()), "the dir stays: a devbox may still bind it")
	entries, err = os.ReadDir(proxyLiveDir())
	require.NoError(t, err)
	assert.Empty(t, entries)
	require.NoError(t, options.OnStop(), "an empty dir is already clean")
}

func TestProxyMap_AllowlistDiff(t *testing.T) {
	testutil.Home(t)

	config := &cfg.Config{CLIName: "claude", Proxy: &cfg.Proxy{Allowlist: []string{"new.example.dev", "kept.example.dev"}}}
	pm := NewProxyMap(config)
	fresh := allowfile.Render(config.AllowlistExpanded())
	changed := func() string {
		r, err := pm.allowlistDiff(fresh)
		require.NoError(t, err)
		return r
	}

	assert.Equal(t, "+kept.example.dev\n+new.example.dev", changed(),
		"a missing file diffs from nothing, in bare domains never the file's regex")

	old := &cfg.Config{CLIName: "claude", Proxy: &cfg.Proxy{Allowlist: []string{"old.example.dev"}}}
	require.NoError(t, NewProxyMap(old).Options(func(string) {}).BeforeStart())
	assert.Equal(t, "-old.example.dev\n+kept.example.dev\n+new.example.dev", changed())

	require.NoError(t, NewProxyMap(config).Options(func(string) {}).OnRefresh())
	assert.Empty(t, changed(), "an unchanged allowlist diffs nothing")

	reordered := &cfg.Config{CLIName: "claude", Proxy: &cfg.Proxy{Allowlist: []string{"kept.example.dev", "new.example.dev"}}}
	require.NoError(t, NewProxyMap(reordered).Options(func(string) {}).BeforeStart())
	assert.Empty(t, changed(), "a reordered allowlist diffs nothing: domain order carries no meaning")

	mixed := &cfg.Config{CLIName: "claude", Proxy: &cfg.Proxy{Allowlist: []string{"kept.example.dev", "gone.example.dev"}}}
	require.NoError(t, NewProxyMap(mixed).Options(func(string) {}).BeforeStart())
	assert.Equal(t, "-gone.example.dev\n+new.example.dev", changed(),
		"a combined reorder, add, and remove renders as sorted changes")
}

func TestProxyMap_Refresh(t *testing.T) {
	testutil.Home(t)

	config := &cfg.Config{CLIName: "claude", Proxy: &cfg.Proxy{Allowlist: []string{"new.example.dev"}}}
	pm := NewProxyMap(config)

	changed, err := pm.Refresh()
	require.NoError(t, err)
	assert.Equal(t, "+new.example.dev", changed, "a missing file diffs from nothing")
	written, err := os.ReadFile(proxyAllowPath())
	require.NoError(t, err)
	assert.Equal(t, allowfile.Render(config.AllowlistExpanded()), written)

	added := &cfg.Config{CLIName: "claude", Proxy: &cfg.Proxy{Allowlist: []string{"new.example.dev", "kept.example.dev"}}}
	changed, err = NewProxyMap(added).Refresh()
	require.NoError(t, err)
	assert.Equal(t, "+kept.example.dev", changed)

	changed, err = NewProxyMap(added).Refresh()
	require.NoError(t, err)
	assert.Empty(t, changed, "a second refresh is a no-op diff")
}

func TestProxyMap_Options_OnRefresh(t *testing.T) {
	testutil.Home(t)

	old := &cfg.Config{CLIName: "claude", Proxy: &cfg.Proxy{Allowlist: []string{"old.example.dev"}}}
	require.NoError(t, NewProxyMap(old).Options(func(string) {}).BeforeStart())

	var reported []string
	config := &cfg.Config{CLIName: "claude", Proxy: &cfg.Proxy{Allowlist: []string{"new.example.dev"}}}
	options := NewProxyMap(config).Options(func(changed string) { reported = append(reported, changed) })
	require.NoError(t, options.OnRefresh())

	written, err := os.ReadFile(proxyAllowPath())
	require.NoError(t, err)
	assert.Equal(t, allowfile.Render(config.AllowlistExpanded()), written)
	require.NoError(t, options.OnRefresh(), "a re-refresh is clean")
	assert.Equal(t, []string{"-old.example.dev\n+new.example.dev"}, reported,
		"a refresh reports its change, staying quiet when unchanged")
}
