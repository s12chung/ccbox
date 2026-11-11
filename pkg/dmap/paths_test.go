package dmap

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestCLIDataDir(t *testing.T) {
	assert.Equal(t, "/home/me/.ccbox/data/opencode", CLIDataDir("/home/me/.ccbox", "opencode"))
}

func TestGUIAppDataDir(t *testing.T) {
	assert.Equal(t, "/home/me/.ccbox/data/zcode/proxy", GUIAppDataDir("/home/me/.ccbox", "zcode", false))
	assert.Equal(t, "/home/me/.ccbox/data/zcode/no-proxy", GUIAppDataDir("/home/me/.ccbox", "zcode", true),
		"the modes seed sibling trees, so a proxied seed never leaks into the plain env")
}

func TestDevboxLogPath(t *testing.T) {
	home := testutil.Home(t)
	assert.Equal(t, filepath.Join(home, ".ccbox", "devbox.log"), devboxLogPath(false, false))
	assert.Equal(t, filepath.Join(home, ".ccbox", "devbox-plain.log"), devboxLogPath(false, true))
	assert.Equal(t, filepath.Join(home, ".ccbox", "devbox-vnc.log"), devboxLogPath(true, true))
	want := filepath.Join(home, ".ccbox", "devbox-vnc-proxy.log")
	assert.Equal(t, want, devboxLogPath(true, false), "the egress wall makes the proxied vnc combo its own variant")
}

func TestProxyLiveDir(t *testing.T) {
	home := testutil.Home(t)
	assert.Equal(t, filepath.Join(home, ".ccbox", "tmp", "proxy"), proxyLiveDir())
}

func TestProxyAllowPath(t *testing.T) {
	home := testutil.Home(t)
	assert.Equal(t, filepath.Join(home, ".ccbox", "tmp", "proxy", "allow.txt"), proxyAllowPath())
}

func TestProxyLogPath(t *testing.T) {
	home := testutil.Home(t)
	assert.Equal(t, filepath.Join(home, ".ccbox", "proxy.log"), proxyLogPath(false))
	assert.Equal(t, filepath.Join(home, ".ccbox", "proxy-vnc-proxy.log"), proxyLogPath(true))
}

func TestWorkspaceMount(t *testing.T) {
	assert.Equal(t, "/home/ccbox/myproj", workspaceMount("/Users/me/myproj"))
}
