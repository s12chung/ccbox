package dmap

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/util/testutil"
)

func TestCLIDataBindPath(t *testing.T) {
	assert.Equal(t, "/home/me/.ccbox/data/opencode/.local-share-opencode-auth.json",
		CLIDataBindPath("/home/me/.ccbox", "opencode", ".local/share/opencode/auth.json"))
}

func TestDevboxLogPath(t *testing.T) {
	home := testutil.Home(t)
	assert.Equal(t, filepath.Join(home, ".ccbox", "devbox.log"), devboxLogPath(false, false))
	assert.Equal(t, filepath.Join(home, ".ccbox", "devbox-plain.log"), devboxLogPath(false, true))
	assert.Equal(t, filepath.Join(home, ".ccbox", "devbox-vnc.log"), devboxLogPath(true, true))
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
	assert.Equal(t, filepath.Join(home, ".ccbox", "proxy-vnc.log"), proxyLogPath(true))
}

func TestWorkspaceMount(t *testing.T) {
	assert.Equal(t, "/home/ccbox/myproj", workspaceMount("/Users/me/myproj"))
}
