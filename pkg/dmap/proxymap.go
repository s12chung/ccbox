package dmap

import (
	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/tinyproxy"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/util/uslice"
)

// ProxyMap maps the project config and the egress wall configs to the docker pkg proxy options
type ProxyMap struct {
	cfg *projectcfg.Config
}

// NewProxyMap returns a new ProxyMap
func NewProxyMap(cfg *projectcfg.Config) *ProxyMap { return &ProxyMap{cfg: cfg} }

// Options renders the egress wall's docker.ProxyOptions; extraAllow appends
// beyond the config's allowlist, skipping entries already listed
func (pm *ProxyMap) Options(extraAllow []string) docker.ProxyOptions {
	allow := pm.cfg.AllowlistExpanded()
	allow = append(allow, uslice.Minus(extraAllow, allow)...)
	return docker.ProxyOptions{
		Config:    tinyproxy.Config,
		Overrides: docker.AllowOverride(allow),
	}
}
