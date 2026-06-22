package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildArgs(t *testing.T) {
	paths := map[string]string{
		"CLAUDE_CONFIG_DIR": configMount,
	}
	assert.Equal(t, paths, buildArgs(BuildOptions{}), "config path is always threaded; version omitted when unset")

	got := buildArgs(BuildOptions{ClaudeCodeVersion: "1.2.3"})
	assert.Equal(t, map[string]string{
		"CLAUDE_CONFIG_DIR":   configMount,
		"CLAUDE_CODE_VERSION": "1.2.3",
	}, got)
}
