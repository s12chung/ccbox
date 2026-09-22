package dmap

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/tinyproxy"
	"github.com/s12chung/ccbox/pkg/projectcfg"
)

func TestProxyMap_Options(t *testing.T) {
	cfg := &projectcfg.Config{
		CLIName:   new("claude"),
		Allowlist: []string{"user.example.dev", projectcfg.DefaultsToken, "example.com"},
	}

	proxyOptions := NewProxyMap(cfg).Options()
	assert.Equal(t, tinyproxy.Config, proxyOptions.Config)
	assert.Equal(t, docker.AllowOverride(cfg.AllowlistExpanded()), proxyOptions.Overrides)
}
