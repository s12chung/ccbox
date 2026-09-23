package dmap

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCLIConfigDir(t *testing.T) {
	userDir := "/home/me/.ccbox"
	assert.Equal(t, filepath.Join(userDir, "claude"), CLIConfigDir(userDir, "claude"))
}

func TestCLIDataBindPath(t *testing.T) {
	assert.Equal(t, "/home/me/.ccbox/data/opencode/.local-share-opencode-auth.json",
		CLIDataBindPath("/home/me/.ccbox", "opencode", ".local/share/opencode/auth.json"))
}

func TestProxyLogPath(t *testing.T) {
	assert.Equal(t, "/home/me/.ccbox/proxy.log", ProxyLogPath("/home/me/.ccbox"))
}

func TestWorkspaceMount(t *testing.T) {
	assert.Equal(t, "/home/ccbox/myproj", workspaceMount("/Users/me/myproj"))
}
