package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/pkg/projectcfg"
)

func TestBuildArgs(t *testing.T) {
	// empty CLI → claude's default config dir; CLI + version omitted when unset
	assert.Equal(t, map[string]string{
		"CONFIG_DIR": claudeConfigMount,
	}, buildArgs(BuildOptions{}))

	got := buildArgs(BuildOptions{CLI: projectcfg.CLICodex, CLIVersion: "1.2.3"})
	assert.Equal(t, map[string]string{
		"CONFIG_DIR":  codexConfigMount,
		"CLI":         "codex",
		"CLI_VERSION": "1.2.3",
	}, got)
}
