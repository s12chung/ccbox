package dmap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCLIDataBindPath(t *testing.T) {
	assert.Equal(t, "/home/me/.ccbox/data/opencode/.local-share-opencode-auth.json",
		CLIDataBindPath("/home/me/.ccbox", "opencode", ".local/share/opencode/auth.json"))
}

func TestProxyLogPath(t *testing.T) {
	assert.Equal(t, "/home/me/.ccbox/proxy.log", proxyLogPath("/home/me/.ccbox"))
}

func TestWorkspaceMount(t *testing.T) {
	assert.Equal(t, "/home/ccbox/myproj", workspaceMount("/Users/me/myproj"))
}
