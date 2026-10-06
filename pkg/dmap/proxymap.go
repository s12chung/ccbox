package dmap

import (
	"path/filepath"

	"github.com/s12chung/ccbox/pkg/docker"
	"github.com/s12chung/ccbox/pkg/kit/tinyproxy"
	"github.com/s12chung/ccbox/pkg/projectcfg"
	"github.com/s12chung/ccbox/pkg/userdir"
	"github.com/s12chung/ccbox/pkg/util/ioutil"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/pkginfo"
	"github.com/s12chung/ccbox/tools/ccboxtools/pkg/proxy"
)

// ProxyMap maps the project config and the egress wall configs to the docker pkg proxy options
type ProxyMap struct {
	cfg *projectcfg.Config
}

// NewProxyMap returns a new ProxyMap
func NewProxyMap(cfg *projectcfg.Config) *ProxyMap { return &ProxyMap{cfg: cfg} }

// Options renders the egress wall's docker.ProxyOptions, its logs streaming through log
func (pm *ProxyMap) Options(log *tinyproxy.Log) *docker.ProxyOptions {
	proxyConfigMap := map[string][]byte{
		tinyproxy.ConfFile:     tinyproxy.MustConf(),
		pkginfo.ProxyAllowFile: proxy.Render(pm.cfg.AllowlistExpanded()),
	}
	return &docker.ProxyOptions{
		HostDir: proxyLiveDir(),
		Log:     log,
		BeforeStart: func() error {
			for name, body := range proxyConfigMap {
				// atomically for reload
				if err := ioutil.AtomicWriteFile(filepath.Join(proxyLiveDir(), name), body); err != nil {
					return err
				}
			}
			return nil
		},
		// contents go, not the dir: a devbox may still bind it
		OnStop: func() error { return ioutil.ClearDir(proxyLiveDir()) },
	}
}

// proxyLiveDir is the proxy's live dir on the host: ~/.ccbox/tmp/proxy. A dir bind tracks
// the changing files as the running proxy's current configs.
func proxyLiveDir() string { return filepath.Join(userdir.Tmp(), "proxy") }

// proxyAllowPath is the rendered allow file within proxyLiveDir
func proxyAllowPath() string { return filepath.Join(proxyLiveDir(), pkginfo.ProxyAllowFile) }
