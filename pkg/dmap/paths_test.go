package dmap

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/pkg/util/slug"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestCLIDataDir(t *testing.T) {
	assert.Equal(t, "/home/me/.ccbox/data/opencode", CLIDataDir("/home/me/.ccbox", "opencode"))
}

func TestGUIAppDataDir(t *testing.T) {
	assert.Equal(t, "/home/me/.ccbox/data/zcode/proxy", GUIAppDataDir("/home/me/.ccbox", "zcode", true))
	assert.Equal(t, "/home/me/.ccbox/data/zcode/no-proxy", GUIAppDataDir("/home/me/.ccbox", "zcode", false),
		"the modes seed sibling trees, so a proxied seed never leaks into the plain env")
}

func TestDevboxLogPath(t *testing.T) {
	home := testutil.Home(t)
	projectDir := "/Users/me/myproj"
	logsDir := filepath.Join(home, ".ccbox", "logs", slug.Path(projectDir))
	assert.Equal(t, filepath.Join(logsDir, "devbox.log"), devboxLogPath(projectDir, false, true))
	assert.Equal(t, filepath.Join(logsDir, "devbox-plain.log"), devboxLogPath(projectDir, false, false))
	assert.Equal(t, filepath.Join(logsDir, "devbox-plain-vnc.log"), devboxLogPath(projectDir, true, false))
	assert.Equal(t, filepath.Join(logsDir, "devbox-vnc.log"), devboxLogPath(projectDir, true, true),
		"the egress wall makes proxied vnc its own variant")
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
	projectDir := "/Users/me/myproj"
	logsDir := filepath.Join(home, ".ccbox", "logs", slug.Path(projectDir))
	assert.Equal(t, filepath.Join(logsDir, "proxy.log"), proxyLogPath(projectDir, false))
	assert.Equal(t, filepath.Join(logsDir, "proxy-vnc.log"), proxyLogPath(projectDir, true))
}

func TestWorkspaceMount(t *testing.T) {
	assert.Equal(t, "/home/ccbox/myproj", workspaceMount("/Users/me/myproj"))
}
