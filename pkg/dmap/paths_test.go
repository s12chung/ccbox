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

func TestProxyLogPath(t *testing.T) {
	home := testutil.Home(t)
	assert.Equal(t, filepath.Join(home, ".ccbox", "proxy.log"), proxyLogPath())
}

func TestWorkspaceMount(t *testing.T) {
	assert.Equal(t, "/home/ccbox/myproj", workspaceMount("/Users/me/myproj"))
}
