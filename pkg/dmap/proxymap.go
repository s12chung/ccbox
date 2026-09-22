package dmap

import (
	"io/fs"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/projectcfg"
)

// ProxyMap maps the project config and the egress wall configs to the docker pkg proxy options
type ProxyMap struct {
	configFS fs.FS // the egress wall configs
	cfg      *projectcfg.Config
}

// NewProxyMap returns a new ProxyMap
func NewProxyMap(configFS fs.FS, cfg *projectcfg.Config) *ProxyMap {
	return &ProxyMap{configFS: configFS, cfg: cfg}
}

// Options renders the egress wall's docker.ProxyOptions
func (pm *ProxyMap) Options() docker.ProxyOptions {
	return docker.ProxyOptions{
		Config:    pm.configFS,
		Overrides: docker.AllowOverride(pm.cfg.AllowlistExpanded()),
	}
}
