package dmap

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/projectcfg"
)

// testProxyConfigFS stands in for the subbed docker/tinyproxy embed
var testProxyConfigFS = fstest.MapFS{"tinyproxy.conf": &fstest.MapFile{Data: []byte("port 8888\n")}}

func TestProxyMap_Options(t *testing.T) {
	cfg := &projectcfg.Config{
		CLIName:   new("claude"),
		Allowlist: []string{"user.example.dev", projectcfg.DefaultsToken, "example.com"},
	}

	proxyOptions := NewProxyMap(testProxyConfigFS, cfg).Options()
	assert.Equal(t, testProxyConfigFS, proxyOptions.Config)
	assert.Equal(t, docker.AllowOverride(cfg.AllowlistExpanded()), proxyOptions.Overrides)
}
