package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvString_BaseWins(t *testing.T) {
	got := envString(map[string]string{
		"GH_TOKEN":   "gh",
		"GOFLAGS":    "-mod=mod",
		"http_proxy": "evil",
	}, false)

	// o.Env sorted, then the wall's proxy vars last — so the base http_proxy is the
	// final (winning) value.
	assert.Equal(t, []string{
		"GH_TOKEN=gh",
		"GOFLAGS=-mod=mod",
		"http_proxy=evil",
		"http_proxy=http://ccbox-egress:8888",
		"https_proxy=http://ccbox-egress:8888",
		"no_proxy=localhost,127.0.0.1,::1",
		"NO_PROXY=localhost,127.0.0.1,::1",
	}, got)
}

func TestEnvString_NoProxy(t *testing.T) {
	got := envString(map[string]string{"GH_TOKEN": "gh"}, true)
	// No wall, no proxy vars — just the caller's env.
	assert.Equal(t, []string{"GH_TOKEN=gh"}, got)
}
