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
		"CONFIG_DIR": path.Join(containerHome, harness.Claude.ConfigMountFolder),
	}, buildArgs(BuildOptions{}))

	got := buildArgs(BuildOptions{CLIName: harness.NameCodex, CLIVersion: "1.2.3"})
	assert.Equal(t, map[string]string{
		"CONFIG_DIR":  path.Join(containerHome, harness.Codex.ConfigMountFolder),
		"CLI":         "codex",
		"CLI_VERSION": "1.2.3",
	}, got)
}
