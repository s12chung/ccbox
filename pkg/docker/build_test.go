package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/pkg/harness"
)

func TestBuildArgs(t *testing.T) {
	// empty CLI → no args; CLI + version omitted when unset
	assert.Equal(t, map[string]string{}, buildArgs(BuildOptions{}))

	// Test with non-Empty BuildOptions, no other cases needed as the logic is simple
	got := buildArgs(BuildOptions{
		CLIName:    harness.NameCodex,
		CLIVersion: "1.2.3",
		Pkger:      "npm:@openai/codex",
	})
	assert.Equal(t, map[string]string{
		"CLI":         "codex",
		"CLI_VERSION": "1.2.3",
		"PKGER":       "npm:@openai/codex",
	}, got)
}
