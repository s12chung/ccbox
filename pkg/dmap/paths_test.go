package dmap

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCLIConfigHostPath(t *testing.T) {
	userDir := "/home/me/.ccbox"
	assert.Equal(t, filepath.Join(userDir, "claude"), CLIConfigHostPath(userDir, "claude"))
}

func TestProjectStateHostPath(t *testing.T) {
	assert.Equal(t, "/home/me/.ccbox/projects/-Users-me-proj",
		ProjectStateHostPath("/home/me/.ccbox", "/Users/me/proj"))
}

func TestCLIDataBindHostPath(t *testing.T) {
	assert.Equal(t, "/home/me/.ccbox/data/opencode/.local-share-opencode-auth.json",
		CLIDataBindHostPath("/home/me/.ccbox", "opencode", ".local/share/opencode/auth.json"))
}

func TestWorkspaceMountPath(t *testing.T) {
	assert.Equal(t, "/home/ccbox/myproj", workspaceMountPath("/Users/me/myproj"))
}

func TestSafeMountPath(t *testing.T) {
	got, err := safeMountPath("/home/ccbox/proj", "vendor/bundle")
	require.NoError(t, err)
	assert.Equal(t, "/home/ccbox/proj/vendor/bundle", got)

	for _, p := range []string{"..", "../escape", "../../x"} {
		_, err := safeMountPath("/home/ccbox/proj", p)
		assert.Error(t, err, p)
	}
}
