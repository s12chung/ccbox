package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvStringBaseWins(t *testing.T) {
	got := envString(RunOptions{
		OAuthToken: "oauth",
		GHToken:    "gh",
		Env:        map[string]string{"GOFLAGS": "-mod=mod", "http_proxy": "evil"},
	})

	// o.Env is sorted and precedes the base vars, so the base http_proxy is the last (winning) value.
	assert.Equal(t, []string{
		"GOFLAGS=-mod=mod",
		"http_proxy=evil",
		"http_proxy=http://ccbox-egress:8888",
		"https_proxy=http://ccbox-egress:8888",
		"CLAUDE_CODE_OAUTH_TOKEN=oauth",
		"GH_TOKEN=gh",
	}, got)
}
