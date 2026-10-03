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

	proxyOptions := NewProxyMap(cfg).Options(nil)
	assert.Equal(t, tinyproxy.Config, proxyOptions.Config)
	assert.Equal(t, docker.AllowOverride(cfg.AllowlistExpanded()), proxyOptions.Overrides)
}

func TestProxyMap_Options_ExtraAllow(t *testing.T) {
	cfg := &projectcfg.Config{
		CLIName:   new("claude"),
		Allowlist: []string{"user.example.dev"},
	}

	proxyOptions := NewProxyMap(cfg).Options([]string{"zcode.z.ai", "user.example.dev"})

	assert.Equal(t, docker.AllowOverride([]string{"user.example.dev", "zcode.z.ai"}), proxyOptions.Overrides)
}
