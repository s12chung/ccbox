package docker

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/pkg/harness"
)

func TestBuildArgs(t *testing.T) {
	// empty CLI → claude's default config dir; CLI + version omitted when unset
	assert.Equal(t, map[string]string{
		"CONFIG_DIR": path.Join(containerHome, harness.Claude.ConfigHomeMount),
	}, buildArgs(BuildOptions{}))

	// Test with non-Empty BuildOptions, no other cases needed as the logic is simple
	got := buildArgs(BuildOptions{CLIName: harness.NameCodex, CLIVersion: "1.2.3"})
	assert.Equal(t, map[string]string{
		"CONFIG_DIR":  path.Join(containerHome, harness.Codex.ConfigHomeMount),
		"CLI":         "codex",
		"CLI_VERSION": "1.2.3",
	}, got)
}
