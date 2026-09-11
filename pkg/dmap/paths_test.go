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

func TestProjectStateDir(t *testing.T) {
	assert.Equal(t, "/home/me/.ccbox/projects/-Users-me-proj",
		ProjectStateDir("/home/me/.ccbox", "/Users/me/proj"))
}

func TestCLIDataBindPath(t *testing.T) {
	assert.Equal(t, "/home/me/.ccbox/data/opencode/.local-share-opencode-auth.json",
		CLIDataBindPath("/home/me/.ccbox", "opencode", ".local/share/opencode/auth.json"))
}

func TestWorkspaceMount(t *testing.T) {
	assert.Equal(t, "/home/ccbox/myproj", workspaceMount("/Users/me/myproj"))
}
